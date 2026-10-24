package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSockets(t *testing.T) {
	s := "header\n 0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 1234\n"
	r, e := ParseSockets(s, "tcp")
	if e != nil || len(r) != 1 || r[0].Address != "127.0.0.1" || r[0].Port != 8080 || r[0].Inode != "1234" {
		t.Fatalf("%+v %v", r, e)
	}
	if _, e := ParseSockets("header\nbad", "tcp"); e == nil {
		t.Fatal("malformed row")
	}
	a, p, e := address("00000000000000000000000001000000:0050")
	if e != nil || a != "::1" || p != 80 {
		t.Fatalf("%s %d %v", a, p, e)
	}
}
func TestRoutes(t *testing.T) {
	r, e := ParseRoutes("Iface Destination Gateway Flags RefCnt Use Metric Mask\neth0 00000000 0102A8C0 0003 0 0 100 00000000\n")
	if e != nil || len(r) != 1 || r[0].Gateway != "192.168.2.1" || r[0].Destination != "0.0.0.0/0" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestCheckLocal(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	r := Check(context.Background(), "127.0.0.1", port, time.Second)
	if r.Outcome != "connected" {
		t.Fatalf("%+v", r)
	}
	l.Close()
	r = Check(context.Background(), "127.0.0.1", port, time.Second)
	if r.Outcome != "refused" {
		t.Fatalf("%+v", r)
	}
}
func TestCancelledCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := Check(ctx, "127.0.0.1", 443, time.Second)
	if r.Outcome == "connected" {
		t.Fatal("ignored cancellation")
	}
}
