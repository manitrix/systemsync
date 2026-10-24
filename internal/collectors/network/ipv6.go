package network

import (
	"encoding/hex"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"net"
	"strconv"
	"strings"
)

func ParseIPv6Routes(s string) ([]types.Route, error) {
	out := []types.Route{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 10 {
			return out, fmt.Errorf("malformed IPv6 route")
		}
		dst, e := hex.DecodeString(f[0])
		if e != nil || len(dst) != 16 {
			return out, fmt.Errorf("invalid IPv6 route destination")
		}
		prefix, e := strconv.ParseUint(f[1], 16, 8)
		if e != nil || prefix > 128 {
			return out, fmt.Errorf("invalid IPv6 prefix")
		}
		gw, e := hex.DecodeString(f[4])
		if e != nil || len(gw) != 16 {
			return out, fmt.Errorf("invalid IPv6 gateway")
		}
		metric, e := strconv.ParseUint(f[5], 16, 32)
		if e != nil {
			return out, e
		}
		out = append(out, types.Route{Interface: f[9], Destination: fmt.Sprintf("%s/%d", net.IP(dst), prefix), Gateway: net.IP(gw).String(), Metric: int(metric), Flags: f[8]})
	}
	return out, nil
}
