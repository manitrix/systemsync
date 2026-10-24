package network

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func address(s string) (string, int, error) {
	a, p, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0, fmt.Errorf("invalid socket endpoint")
	}
	b, e := hex.DecodeString(a)
	if e != nil {
		return "", 0, e
	}
	if len(b) != 4 && len(b) != 16 {
		return "", 0, fmt.Errorf("invalid IP length")
	}
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+3] = b[i+3], b[i]
		b[i+1], b[i+2] = b[i+2], b[i+1]
	}
	port, e := strconv.ParseUint(p, 16, 16)
	return net.IP(b).String(), int(port), e
}
func ParseSockets(s, protocol string) ([]types.Socket, error) {
	out := []types.Socket{}
	scan := bufio.NewScanner(strings.NewReader(s))
	first := true
	for scan.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(scan.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) < 10 {
			return out, fmt.Errorf("malformed %s socket row", protocol)
		}
		ip, port, e := address(f[1])
		if e != nil {
			return out, e
		}
		if strings.HasPrefix(protocol, "tcp") && f[3] != "0A" {
			continue
		}
		out = append(out, types.Socket{Protocol: protocol, Address: ip, Port: port, State: f[3], Inode: f[9]})
	}
	return out, scan.Err()
}
func ParseRoutes(s string) ([]types.Route, error) {
	out := []types.Route{}
	for i, l := range strings.Split(s, "\n") {
		if i == 0 || strings.TrimSpace(l) == "" {
			continue
		}
		f := strings.Fields(l)
		if len(f) < 8 {
			return out, fmt.Errorf("malformed IPv4 route")
		}
		dst, e := strconv.ParseUint(f[1], 16, 32)
		if e != nil {
			return out, e
		}
		gw, e := strconv.ParseUint(f[2], 16, 32)
		if e != nil {
			return out, e
		}
		mask, e := strconv.ParseUint(f[7], 16, 32)
		if e != nil {
			return out, e
		}
		ip := func(n uint64) net.IP {
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(n))
			return net.IP(b)
		}
		ones, _ := net.IPMask(ip(mask)).Size()
		metric, e := strconv.Atoi(f[6])
		if e != nil {
			return out, e
		}
		out = append(out, types.Route{Interface: f[0], Destination: fmt.Sprintf("%s/%d", ip(dst), ones), Gateway: ip(gw).String(), Flags: f[3], Metric: metric})
	}
	return out, nil
}
func Collect(ctx context.Context) (types.Network, error) {
	var n types.Network
	if runtime.GOOS != "linux" {
		return n, fmt.Errorf("network snapshots require Linux")
	}
	ifs, e := net.Interfaces()
	if e != nil {
		return n, e
	}
	for _, i := range ifs {
		if e := ctx.Err(); e != nil {
			return n, e
		}
		state := "down"
		if i.Flags&net.FlagUp != 0 {
			state = "up"
		}
		item := types.Interface{Name: i.Name, State: state, MTU: i.MTU}
		a, err := i.Addrs()
		if err != nil {
			n.Warnings = append(n.Warnings, err.Error())
		}
		for _, v := range a {
			item.Addresses = append(item.Addresses, v.String())
		}
		n.Interfaces = append(n.Interfaces, item)
	}
	for _, proto := range []string{"tcp", "tcp6", "udp", "udp6"} {
		b, err := os.ReadFile("/proc/net/" + proto)
		if err != nil {
			n.Warnings = append(n.Warnings, err.Error())
			continue
		}
		rows, err := ParseSockets(string(b), proto)
		if err != nil {
			n.Warnings = append(n.Warnings, err.Error())
			continue
		}
		n.Sockets = append(n.Sockets, rows...)
	}
	b, e := os.ReadFile("/proc/net/route")
	if e != nil {
		n.Warnings = append(n.Warnings, e.Error())
	} else {
		n.Routes, e = ParseRoutes(string(b))
		if e != nil {
			n.Warnings = append(n.Warnings, e.Error())
		}
	}
	b, e = os.ReadFile("/proc/net/ipv6_route")
	if e != nil {
		n.Warnings = append(n.Warnings, e.Error())
	} else {
		routes, err := ParseIPv6Routes(string(b))
		if err != nil {
			n.Warnings = append(n.Warnings, err.Error())
		} else {
			n.Routes = append(n.Routes, routes...)
		}
	}
	b, e = os.ReadFile("/etc/resolv.conf")
	if e != nil {
		n.Warnings = append(n.Warnings, e.Error())
	} else {
		n.Resolver = string(b)
	}
	n.Namespace, e = os.Readlink("/proc/self/ns/net")
	if e != nil {
		n.Warnings = append(n.Warnings, e.Error())
	}
	return n, nil
}
func Check(ctx context.Context, host string, port int, timeout time.Duration) types.Check {
	start := time.Now()
	r := types.Check{Host: host, Port: port}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ips, e := net.DefaultResolver.LookupHost(ctx, host)
	if e != nil {
		r.Outcome = "dns_failure"
		r.Detail = e.Error()
		r.Duration = time.Since(start)
		return r
	}
	r.Addresses = ips
	if port == 0 {
		r.Outcome = "resolved"
		r.Duration = time.Since(start)
		return r
	}
	conn, e := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if e == nil {
		conn.Close()
		r.Outcome = "connected"
	} else {
		r.Detail = e.Error()
		var ne net.Error
		switch {
		case errors.Is(e, syscall.ECONNREFUSED), runtime.GOOS == "windows" && errors.Is(e, syscall.Errno(10061)):
			r.Outcome = "refused"
		case errors.Is(e, syscall.ENETUNREACH), errors.Is(e, syscall.EHOSTUNREACH), runtime.GOOS == "windows" && (errors.Is(e, syscall.Errno(10051)) || errors.Is(e, syscall.Errno(10065))):
			r.Outcome = "unreachable"
		case errors.As(e, &ne) && ne.Timeout():
			r.Outcome = "timeout"
		default:
			r.Outcome = "error"
		}
	}
	r.Duration = time.Since(start)
	return r
}
func Firewall(ctx context.Context) (map[string]any, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("firewall inspection requires Linux")
	}
	result := map[string]any{}
	if p, e := exec.LookPath("iptables"); e == nil {
		b, err := exec.CommandContext(ctx, p, "--version").CombinedOutput()
		result["iptables_version"] = strings.TrimSpace(string(b))
		if err != nil {
			result["iptables_error"] = err.Error()
		}
	}
	p, e := exec.LookPath("nft")
	if e != nil {
		return result, fmt.Errorf("nft unavailable; install nftables to inspect the ruleset")
	}
	b, e := exec.CommandContext(ctx, p, "--json", "list", "ruleset").Output()
	if e != nil {
		return result, fmt.Errorf("nft ruleset inaccessible (CAP_NET_ADMIN may be required): %w", e)
	}
	result["nft_json"] = string(b)
	return result, nil
}
