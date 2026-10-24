package network

import "testing"

func TestIPv6Routes(t *testing.T) {
	r, e := ParseIPv6Routes("00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000064 00000000 00000000 00000003 eth0\n")
	if e != nil || len(r) != 1 || r[0].Destination != "::/0" || r[0].Gateway != "fe80::1" || r[0].Metric != 100 {
		t.Fatalf("%+v %v", r, e)
	}
}
