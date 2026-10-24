//go:build linux

package system

import (
	"context"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Collect(ctx context.Context, interval time.Duration) (types.System, error) {
	var s types.System
	s.CPUCount = runtime.NumCPU()
	read := func(p string) (string, error) { b, e := os.ReadFile(p); return string(b), e }
	a, e := read("/proc/stat")
	if e != nil {
		return s, e
	}
	first, e := ParseCPU(a)
	if e != nil {
		return s, e
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return s, ctx.Err()
	case <-timer.C:
	}
	b, e := read("/proc/stat")
	if e != nil {
		return s, e
	}
	second, e := ParseCPU(b)
	if e != nil {
		return s, e
	}
	s.CPUPercent, e = CPUPercent(first, second)
	if e != nil {
		return s, e
	}
	mem, e := read("/proc/meminfo")
	if e != nil {
		return s, e
	}
	m, e := ParseMemory(mem)
	if e != nil {
		return s, e
	}
	s.MemoryTotal = m["MemTotal"]
	s.MemoryAvailable = m["MemAvailable"]
	s.SwapTotal = m["SwapTotal"]
	s.SwapFree = m["SwapFree"]
	if s.Kernel, e = read("/proc/sys/kernel/osrelease"); e != nil {
		return s, e
	}
	s.Kernel = strings.TrimSpace(s.Kernel)
	s.OS, _ = read("/etc/os-release")
	up, e := read("/proc/uptime")
	if e != nil {
		return s, e
	}
	uf := strings.Fields(up)
	if len(uf) == 0 {
		return s, fmt.Errorf("empty uptime")
	}
	s.Uptime, e = strconv.ParseFloat(uf[0], 64)
	if e != nil {
		return s, e
	}
	load, e := read("/proc/loadavg")
	if e != nil {
		return s, e
	}
	lf := strings.Fields(load)
	if len(lf) < 3 {
		return s, fmt.Errorf("invalid loadavg")
	}
	for _, v := range lf[:3] {
		n, e := strconv.ParseFloat(v, 64)
		if e != nil {
			return s, e
		}
		s.Load = append(s.Load, n)
	}
	mounts, e := read("/proc/self/mounts")
	if e != nil {
		return s, e
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(mounts, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[2] {
		case "proc", "sysfs", "tmpfs", "devtmpfs", "cgroup", "cgroup2", "devpts", "securityfs", "debugfs", "tracefs", "pstore", "mqueue", "hugetlbfs", "fusectl", "configfs":
			continue
		}
		mount := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\").Replace(f[1])
		if seen[mount] {
			continue
		}
		seen[mount] = true
		var st syscall.Statfs_t
		if e := ctx.Err(); e != nil {
			return s, e
		}
		if e := syscall.Statfs(mount, &st); e != nil {
			s.Warnings = append(s.Warnings, fmt.Sprintf("%s: %v", mount, e))
			continue
		}
		if st.Blocks == 0 {
			continue
		}
		s.Disks = append(s.Disks, types.Disk{Mount: mount, Total: st.Blocks * uint64(st.Bsize), Available: st.Bavail * uint64(st.Bsize), Inodes: st.Files, FreeInodes: st.Ffree})
	}
	return s, nil
}
