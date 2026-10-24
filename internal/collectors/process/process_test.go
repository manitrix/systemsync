package process

import "testing"

func TestStatus(t *testing.T) {
	p, e := ParseStatus(42, "Name:\tworker\nState:\tS (sleeping)\nVmRSS:\t2048 kB\nThreads:\t3\n")
	if e != nil || p.RSS != 2097152 || p.FDs != -1 || p.Threads != 3 {
		t.Fatalf("%+v %v", p, e)
	}
	if _, e := ParseStatus(1, "VmRSS: bad kB"); e == nil {
		t.Fatal("accepted malformed status")
	}
}
