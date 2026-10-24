package rules

import (
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/pkg/types"
	"strings"
	"testing"
	"time"
)

func TestResourceEvidence(t *testing.T) {
	now := time.Now().UTC()
	o := types.Observation{Timestamp: now, Data: types.System{MemoryTotal: 1000, MemoryAvailable: 10, CPUCount: 2, CPUPercent: 95, Load: []float64{3, 3, 2}, Disks: []types.Disk{{Mount: "/", Total: 1000, Available: 1, Inodes: 100, FreeInodes: 1}}}}
	fs := Default().Evaluate([]types.Observation{o}, config.Default())
	if len(fs) != 4 {
		t.Fatalf("%+v", fs)
	}
	for _, f := range fs {
		if len(f.Evidence) == 0 || len(f.Steps) == 0 || len(f.Causes) == 0 || f.Timestamp != now || f.Confidence == "" {
			t.Fatalf("incomplete finding %+v", f)
		}
	}
}
func TestNoFalseOOMOrFirewall(t *testing.T) {
	obs := []types.Observation{{Data: types.Service{Name: "web", Active: "failed", ExecStatus: 137}}, {Data: types.Check{Host: "127.0.0.1", Port: 80, Outcome: "timeout"}}}
	fs := Default().Evaluate(obs, config.Default())
	if len(fs) != 2 {
		t.Fatalf("%+v", fs)
	}
	for _, f := range fs {
		if strings.Contains(f.ID, "oom") {
			t.Fatal("exit 137 alone does not prove OOM")
		}
		if strings.Contains(f.ID, "timeout") && !strings.Contains(f.Explanation, "unverified") {
			t.Fatal("timeout must not establish firewall cause")
		}
	}
}
func TestContainerAndService(t *testing.T) {
	obs := []types.Observation{{Data: types.Container{ID: "a", Status: "exited", ExitCode: 1, OOMKilled: true, Restarts: 4, Health: "unhealthy", MemoryUsage: 95, MemoryLimit: 100}}, {Data: types.Service{Name: "x", Result: "oom-kill", Logs: []string{"bind: address already in use"}}}}
	fs := Default().Evaluate(obs, config.Default())
	if len(fs) != 7 {
		t.Fatalf("got %d: %+v", len(fs), fs)
	}
}
func TestRuleRegistration(t *testing.T) {
	e := Engine{Rules: []Rule{RuleFunc(func(types.Observation, config.Config) []types.Finding { return []types.Finding{{ID: "custom"}} })}}
	if len(e.Evaluate([]types.Observation{{}}, config.Default())) != 1 {
		t.Fatal("registration")
	}
}
