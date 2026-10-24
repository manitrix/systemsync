package system

import (
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"strconv"
	"strings"
)

func ParseCPU(s string) (types.CPU, error) {
	var c types.CPU
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return c, fmt.Errorf("empty stat")
	}
	f := strings.Fields(lines[0])
	if len(f) < 5 || f[0] != "cpu" {
		return c, fmt.Errorf("invalid aggregate cpu counters")
	}
	for i, v := range f[1:] {
		if i >= 8 {
			break
		}
		n, e := strconv.ParseUint(v, 10, 64)
		if e != nil {
			return c, e
		}
		c.Total += n
		if i != 3 && i != 4 {
			c.Busy += n
		}
	}
	return c, nil
}
func CPUPercent(a, b types.CPU) (float64, error) {
	if b.Total <= a.Total || b.Busy < a.Busy {
		return 0, fmt.Errorf("cpu counters did not advance or reset")
	}
	return float64(b.Busy-a.Busy) / float64(b.Total-a.Total) * 100, nil
}
func ParseMemory(s string) (map[string]uint64, error) {
	m := map[string]uint64{}
	for _, l := range strings.Split(s, "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		n, e := strconv.ParseUint(f[1], 10, 64)
		if e != nil {
			return nil, e
		}
		if len(f) > 2 && f[2] == "kB" {
			n *= 1024
		}
		m[strings.TrimSuffix(f[0], ":")] = n
	}
	if m["MemTotal"] == 0 {
		return nil, fmt.Errorf("MemTotal missing")
	}
	if _, ok := m["MemAvailable"]; !ok {
		return nil, fmt.Errorf("MemAvailable missing (Linux 3.14+ required)")
	}
	return m, nil
}
