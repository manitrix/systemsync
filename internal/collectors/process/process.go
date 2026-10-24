package process

import (
	"context"
	"fmt"
	"github.com/manitrix/systemsync/internal/collectors/system"
	"github.com/manitrix/systemsync/pkg/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ParseStatus(pid int, s string) (types.Process, error) {
	p := types.Process{PID: pid, FDs: -1}
	for _, l := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "Name":
			p.Name = v
		case "State":
			p.State = v
		case "VmRSS":
			f := strings.Fields(v)
			if len(f) == 0 {
				return p, fmt.Errorf("invalid VmRSS")
			}
			n, e := strconv.ParseUint(f[0], 10, 64)
			if e != nil {
				return p, e
			}
			p.RSS = n * 1024
		case "Threads":
			n, e := strconv.Atoi(v)
			if e != nil {
				return p, e
			}
			p.Threads = n
		}
	}
	if p.Name == "" {
		return p, fmt.Errorf("missing process name")
	}
	return p, nil
}
func Collect(ctx context.Context, pid int, interval time.Duration) ([]types.Process, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("process collector requires Linux")
	}
	entries, e := os.ReadDir("/proc")
	if e != nil {
		return nil, e
	}
	out := []types.Process{}
	starts := map[int]uint64{}
	statBefore, statErr := os.ReadFile("/proc/stat")
	before, parseErr := system.ParseCPU(string(statBefore))
	for _, entry := range entries {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		n, e := strconv.Atoi(entry.Name())
		if e != nil || pid > 0 && n != pid {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		b, e := os.ReadFile(filepath.Join(base, "status"))
		if e != nil {
			if pid > 0 {
				return out, e
			}
			continue
		}
		p, e := ParseStatus(n, string(b))
		if e != nil {
			if pid > 0 {
				return out, e
			}
			continue
		}
		fds, e := os.ReadDir(filepath.Join(base, "fd"))
		if e == nil {
			p.FDs = len(fds)
		} else {
			p.Warnings = append(p.Warnings, "fd count: "+e.Error())
		}
		b, e = os.ReadFile(filepath.Join(base, "stat"))
		if e == nil {
			p.CPUTicks, starts[n], e = ParseStat(string(b))
		}
		if e != nil {
			p.Warnings = append(p.Warnings, "CPU stat: "+e.Error())
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	if pid > 0 && len(out) == 0 {
		return out, fmt.Errorf("PID %d not found or inaccessible", pid)
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return out, ctx.Err()
	case <-timer.C:
	}
	statAfter, err := os.ReadFile("/proc/stat")
	after, parseAfter := system.ParseCPU(string(statAfter))
	for i := range out {
		p := &out[i]
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		b, e := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.PID), "stat"))
		if e != nil {
			p.Warnings = append(p.Warnings, "CPU second sample: "+e.Error())
			continue
		}
		ticks, start, e := ParseStat(string(b))
		if e != nil || start != starts[p.PID] || ticks < p.CPUTicks || statErr != nil || parseErr != nil || err != nil || parseAfter != nil || after.Total <= before.Total {
			p.Warnings = append(p.Warnings, "CPU sampling unavailable: counters reset, PID changed, or inaccessible stat")
			continue
		}
		v := float64(ticks-p.CPUTicks) / float64(after.Total-before.Total) * float64(runtime.NumCPU()) * 100
		p.CPUPercent = &v
		p.CPUTicks = ticks
	}
	return out, nil
}

// ParseStat handles spaces and parentheses in comm by finding the final closing parenthesis.
func ParseStat(s string) (ticks, start uint64, err error) {
	end := strings.LastIndex(s, ")")
	if end < 0 {
		return 0, 0, fmt.Errorf("invalid process stat comm")
	}
	f := strings.Fields(s[end+1:])
	if len(f) < 20 {
		return 0, 0, fmt.Errorf("short process stat")
	}
	user, e := strconv.ParseUint(f[11], 10, 64)
	if e != nil {
		return 0, 0, e
	}
	kernel, e := strconv.ParseUint(f[12], 10, 64)
	if e != nil {
		return 0, 0, e
	}
	start, e = strconv.ParseUint(f[19], 10, 64)
	return user + kernel, start, e
}
