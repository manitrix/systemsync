package rules

import (
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/pkg/types"
	"strings"
	"testing"
)

func TestCorrelatedBindConflict(t *testing.T) {
	obs := []types.Observation{{Data: types.Service{Name: "web", Active: "failed", Logs: []string{"bind() to 0.0.0.0:8080 failed: Address already in use"}}}, {Data: types.Network{Sockets: []types.Socket{{Protocol: "tcp", Port: 8080, Address: "0.0.0.0", Inode: "123"}}}}}
	fs := Correlate(Default().Evaluate(obs, config.Default()), obs)
	for _, f := range fs {
		if strings.HasPrefix(f.ID, "service.bind:") {
			if len(f.Evidence) != 2 || f.Confidence != "probable" {
				t.Fatalf("%+v", f)
			}
			return
		}
	}
	t.Fatal("missing correlated finding")
}
