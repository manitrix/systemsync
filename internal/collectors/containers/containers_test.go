package containers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAdapters(t *testing.T) {
	for _, kind := range []string{"docker", "podman"} {
		t.Run(kind, func(t *testing.T) {
			prefix := "/v1.41"
			if kind == "podman" {
				prefix = "/v4.0.0/libpod"
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, prefix) {
					t.Errorf("wrong prefix %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/containers/json"):
					fmt.Fprint(w, `[{"Id":"abc","Names":["/web"],"State":"running"}]`)
				case strings.HasSuffix(r.URL.Path, "/abc/json"):
					fmt.Fprint(w, `{"Id":"abc","Name":"/web","RestartCount":4,"State":{"Status":"running","OOMKilled":true,"Health":{"Status":"unhealthy"},"Healthcheck":{"Status":"unhealthy"}},"HostConfig":{"Memory":1000},"Mounts":[]}`)
				case strings.HasSuffix(r.URL.Path, "/logs"):
					fmt.Fprint(w, "hello\n")
				case strings.HasSuffix(r.URL.Path, "/stats"):
					if kind == "podman" {
						if r.URL.Query().Get("containers") != "abc" {
							t.Error("missing native stats target")
						}
						fmt.Fprint(w, `{"Stats":[{"ContainerID":"abc","Name":"web","CPU":12.5,"MemUsage":950}],"Error":null}`)
					} else {
						fmt.Fprint(w, `{"memory_stats":{"usage":950}}`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			base := srv.Client().Transport
			a := &API{Kind: kind, Prefix: prefix, LogLimit: 5, Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme = "http"
				r.URL.Host = strings.TrimPrefix(srv.URL, "http://")
				return base.RoundTrip(r)
			})}}
			rows, e := a.List(context.Background())
			if e != nil || len(rows) != 1 || rows[0].Name != "web" {
				t.Fatalf("%+v %v", rows, e)
			}
			c, e := a.Inspect(context.Background(), "abc")
			if e != nil || !c.OOMKilled || c.Health != "unhealthy" || c.Logs != "hello\n" || c.MemoryUsage != 950 || len(c.Warnings) > 0 {
				t.Fatalf("%+v %v", c, e)
			}
			if _, e = a.Inspect(context.Background(), "missing"); e == nil {
				t.Fatal("ignored 404")
			}
		})
	}
}
func TestDecodeLogs(t *testing.T) {
	b := append([]byte{1, 0, 0, 0, 0, 0, 0, 3}, []byte("foo")...)
	if DecodeLogs(b) != "foo" {
		t.Fatal("frame decode")
	}
	if DecodeLogs([]byte("raw")) != "raw" {
		t.Fatal("raw decode")
	}
}
