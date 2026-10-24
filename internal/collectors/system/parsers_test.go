package system

import (
	"math"
	"testing"
)

func TestCPUSampling(t *testing.T) {
	a, e := ParseCPU("cpu 100 0 50 800 50 0 0 0 100 0\n")
	if e != nil {
		t.Fatal(e)
	}
	b, e := ParseCPU("cpu 150 0 50 840 60 0 0 0 150 0\n")
	if e != nil {
		t.Fatal(e)
	}
	v, e := CPUPercent(a, b)
	if e != nil || math.Abs(v-50) > 0.01 {
		t.Fatalf("CPU=%v %v", v, e)
	}
	if _, e = CPUPercent(b, a); e == nil {
		t.Fatal("accepted reset")
	}
	if _, e = ParseCPU("cpu bad 0 0 0"); e == nil {
		t.Fatal("accepted malformed counter")
	}
}
func TestMemory(t *testing.T) {
	m, e := ParseMemory("MemTotal: 1024 kB\nMemAvailable: 512 kB\nSwapTotal: 0 kB\n")
	if e != nil || m["MemAvailable"] != 524288 {
		t.Fatalf("%v %v", m, e)
	}
	for _, s := range []string{"", "MemTotal: no kB", "MemTotal: 1 kB"} {
		if _, e := ParseMemory(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
