// Package containers talks to local runtime Unix sockets only. It never starts a runtime.
package containers

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Runtime interface {
	Name() string
	List(context.Context) ([]types.Container, error)
	Inspect(context.Context, string) (types.Container, error)
}
type API struct {
	Kind     string
	Prefix   string
	Client   *http.Client
	LogLimit int
}
type Docker struct{ *API }
type Podman struct{ *API }

func (a *API) Name() string { return a.Kind }
func Local(kind, socket string, limit int) *API {
	prefix := ""
	if kind == "podman" {
		prefix = "/v4.0.0/libpod"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &API{Kind: kind, Prefix: prefix, Client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("runtime redirects disabled") }}, LogLimit: limit}
}
func Available(limit int) []Runtime {
	docker := os.Getenv("SSYNC_DOCKER_SOCKET")
	if docker == "" {
		docker = "/var/run/docker.sock"
	}
	podman := os.Getenv("SSYNC_PODMAN_SOCKET")
	if podman == "" {
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir != "" {
			podman = filepath.Join(dir, "podman", "podman.sock")
		} else {
			podman = "/run/podman/podman.sock"
		}
	}
	return []Runtime{Docker{Local("docker", docker, limit)}, Podman{Local("podman", podman, limit)}}
}
func (a *API) get(ctx context.Context, path string) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", "http://localhost"+a.Prefix+path, nil)
	if e != nil {
		return nil, e
	}
	res, e := a.Client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("%s API unavailable: %w", a.Kind, e)
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 4*1024*1024 {
		return nil, fmt.Errorf("%s response exceeds 4 MiB", a.Kind)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s API returned HTTP %d", a.Kind, res.StatusCode)
	}
	return b, nil
}
func (a *API) List(ctx context.Context) ([]types.Container, error) {
	a, err := a.versioned(ctx)
	if err != nil {
		return nil, err
	}
	b, e := a.get(ctx, "/containers/json?all=true")
	if e != nil {
		return nil, e
	}
	var rows []struct {
		ID     string `json:"Id"`
		Names  []string
		State  string
		Status string
		Ports  any
	}
	if e = json.Unmarshal(b, &rows); e != nil {
		return nil, e
	}
	if rows == nil {
		return nil, fmt.Errorf("runtime returned null instead of a container list")
	}
	out := []types.Container{}
	for _, r := range rows {
		name := r.ID
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		out = append(out, types.Container{ID: r.ID, Name: name, Runtime: a.Kind, Status: r.State, Ports: r.Ports})
	}
	return out, nil
}
func (a *API) Inspect(ctx context.Context, id string) (types.Container, error) {
	var c types.Container
	if id == "" || strings.HasPrefix(id, "-") {
		return c, fmt.Errorf("container ID required")
	}
	a, err := a.versioned(ctx)
	if err != nil {
		return c, err
	}
	path := "/containers/" + url.PathEscape(id)
	b, e := a.get(ctx, path+"/json")
	if e != nil {
		return c, e
	}
	var r struct {
		ID           string `json:"Id"`
		Name         string
		RestartCount int
		State        struct {
			Status      string
			ExitCode    int
			OOMKilled   bool
			Health      struct{ Status string } `json:"Health"`
			Healthcheck struct{ Status string } `json:"Healthcheck"`
		}
		HostConfig      struct{ Memory uint64 }
		NetworkSettings struct {
			Ports    any
			Networks any
		}
		Mounts any
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return c, e
	}
	if r.ID == "" || r.State.Status == "" {
		return c, fmt.Errorf("runtime inspect response omitted ID or state")
	}
	health := r.State.Health.Status
	if a.Kind == "podman" && r.State.Healthcheck.Status != "" {
		health = r.State.Healthcheck.Status
	}
	c = types.Container{ID: r.ID, Name: strings.TrimPrefix(r.Name, "/"), Runtime: a.Kind, Status: r.State.Status, ExitCode: r.State.ExitCode, OOMKilled: r.State.OOMKilled, Health: health, Restarts: r.RestartCount, MemoryLimit: r.HostConfig.Memory, Ports: r.NetworkSettings.Ports, Networks: r.NetworkSettings.Networks, Mounts: r.Mounts}
	logs, err := a.get(ctx, fmt.Sprintf("%s/logs?stdout=true&stderr=true&tail=%d", path, a.LogLimit))
	if err != nil {
		c.Warnings = append(c.Warnings, err.Error())
	} else {
		c.Logs = DecodeLogs(logs)
	}
	// Podman's native stats response has a different schema; do not decode it as Docker stats.
	if a.Kind == "docker" {
		b, err = a.get(ctx, path+"/stats?stream=false")
		if err != nil {
			c.Warnings = append(c.Warnings, err.Error())
		} else {
			var stats struct {
				Memory struct {
					Usage uint64 `json:"usage"`
				} `json:"memory_stats"`
				CPU CPUStats `json:"cpu_stats"`
				Pre CPUStats `json:"precpu_stats"`
			}
			if err = json.Unmarshal(b, &stats); err != nil {
				c.Warnings = append(c.Warnings, err.Error())
			} else {
				c.MemoryUsage = stats.Memory.Usage
				if stats.CPU.System > stats.Pre.System && stats.CPU.Usage.Total >= stats.Pre.Usage.Total {
					c.CPUPercent = float64(stats.CPU.Usage.Total-stats.Pre.Usage.Total) / float64(stats.CPU.System-stats.Pre.System) * float64(stats.CPU.Online) * 100
				}
			}
		}
	} else {
		b, err = a.get(ctx, "/containers/stats?containers="+url.QueryEscape(id)+"&stream=false&interval=1")
		if err != nil {
			c.Warnings = append(c.Warnings, err.Error())
		} else {
			var stats struct {
				Error json.RawMessage
				Stats []struct {
					ContainerID string
					Name        string
					CPU         float64
					MemUsage    uint64
				}
			}
			if err = json.Unmarshal(b, &stats); err != nil {
				c.Warnings = append(c.Warnings, err.Error())
			} else if len(stats.Error) > 0 && string(stats.Error) != "null" {
				c.Warnings = append(c.Warnings, "Podman stats reported an error")
			} else {
				matched := false
				for _, sample := range stats.Stats {
					if sample.ContainerID == c.ID || sample.Name == c.Name {
						c.MemoryUsage = sample.MemUsage
						c.CPUPercent = sample.CPU
						matched = true
						break
					}
				}
				if !matched {
					c.Warnings = append(c.Warnings, "Podman stats returned no matching container sample")
				}
			}
		}
	}
	return c, nil
}

// Query the daemon's supported API version instead of assuming an old version is accepted.
// A copy avoids shared mutable version state when an adapter is reused concurrently.
func (a *API) versioned(ctx context.Context) (*API, error) {
	if a.Prefix != "" {
		return a, nil
	}
	b, e := a.get(ctx, "/version")
	if e != nil {
		return nil, e
	}
	var v struct {
		APIVersion string `json:"ApiVersion"`
	}
	if e = json.Unmarshal(b, &v); e != nil {
		return nil, e
	}
	if !regexp.MustCompile(`^1\.[0-9]+$`).MatchString(v.APIVersion) {
		return nil, fmt.Errorf("unsupported Docker API version %q", v.APIVersion)
	}
	copy := *a
	copy.Prefix = "/v" + v.APIVersion
	return &copy, nil
}

type CPUStats struct {
	System uint64 `json:"system_cpu_usage"`
	Online uint64 `json:"online_cpus"`
	Usage  struct {
		Total uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
}

func DecodeLogs(b []byte) string {
	var out strings.Builder
	for len(b) >= 8 && b[0] <= 2 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		n := int(binary.BigEndian.Uint32(b[4:8]))
		if n > len(b)-8 {
			return out.String() + "[truncated log frame]"
		}
		out.Write(b[8 : 8+n])
		b = b[8+n:]
	}
	out.Write(b)
	return out.String()
}
