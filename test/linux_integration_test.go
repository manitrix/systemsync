//go:build integration && linux

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/internal/collectors/containers"
	"github.com/manitrix/systemsync/internal/collectors/network"
	"github.com/manitrix/systemsync/internal/collectors/process"
	"github.com/manitrix/systemsync/internal/collectors/system"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveUnprivilegedCollectors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, e := system.Collect(ctx, 100*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	if s.MemoryTotal == 0 || s.Kernel == "" || s.Uptime <= 0 || s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Fatalf("invalid live system observation: %+v", s)
	}
	processes, e := process.Collect(ctx, os.Getpid(), 100*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	if len(processes) != 1 || processes[0].PID != os.Getpid() || processes[0].CPUPercent == nil {
		t.Fatalf("process observation: %+v", processes)
	}
	n, e := network.Collect(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if len(n.Interfaces) == 0 || n.Namespace == "" {
		t.Fatalf("invalid network observation %+v", n)
	}
	if _, e = json.Marshal(n); e != nil {
		t.Fatal(e)
	}
}

func TestRuntimeUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "runtime.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("runtime adapter attempted a mutation")
		}
		switch r.URL.Path {
		case "/version":
			fmt.Fprint(w, `{"ApiVersion":"1.53"}`)
		case "/v1.53/containers/json":
			fmt.Fprint(w, `[{"Id":"fixture","Names":["/web"],"State":"running"}]`)
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})}
	go srv.Serve(l)
	defer srv.Close()
	a := containers.Local("docker", socket, 10)
	rows, e := a.List(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[0].ID != "fixture" {
		t.Fatalf("%+v", rows)
	}
	// A missing socket is an explicit error, not an empty healthy runtime.
	a = containers.Local("podman", socket+"-missing", 10)
	if _, e = a.List(context.Background()); e == nil || !strings.Contains(e.Error(), "unavailable") {
		t.Fatalf("missing socket error: %v", e)
	}
}
