// Package rules implements deterministic, independently registerable evidence rules.
package rules

import (
	"fmt"
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/pkg/types"
	"strings"
)

type Rule interface {
	Evaluate(types.Observation, config.Config) []types.Finding
}
type RuleFunc func(types.Observation, config.Config) []types.Finding

func (f RuleFunc) Evaluate(o types.Observation, c config.Config) []types.Finding { return f(o, c) }

type Engine struct{ Rules []Rule }

func Default() Engine {
	return Engine{Rules: []Rule{RuleFunc(resources), RuleFunc(service), RuleFunc(connectivity), RuleFunc(container)}}
}
func (e Engine) Evaluate(obs []types.Observation, c config.Config) []types.Finding {
	out := []types.Finding{}
	for _, o := range obs {
		switch rows := o.Data.(type) {
		case []types.Service:
			for _, row := range rows {
				child := o
				child.Data = row
				out = append(out, e.Evaluate([]types.Observation{child}, c)...)
			}
			continue
		case []types.Container:
			for _, row := range rows {
				child := o
				child.Data = row
				out = append(out, e.Evaluate([]types.Observation{child}, c)...)
			}
			continue
		}
		for _, rule := range e.Rules {
			for _, f := range rule.Evaluate(o, c) {
				f.Timestamp = o.Timestamp
				out = append(out, f)
			}
		}
	}
	return out
}
func finding(id, category, severity, confidence, title, explanation string, evidence, causes, steps []string) types.Finding {
	return types.Finding{ID: id, Category: category, Severity: severity, Confidence: confidence, Title: title, Explanation: explanation, Evidence: evidence, Causes: causes, Steps: steps}
}
func resources(o types.Observation, c config.Config) []types.Finding {
	s, ok := o.Data.(types.System)
	if !ok {
		return nil
	}
	out := []types.Finding{}
	if s.MemoryTotal > 0 {
		used := 100 * (1 - float64(s.MemoryAvailable)/float64(s.MemoryTotal))
		if used >= c.MemoryPercent {
			out = append(out, finding("memory.pressure", "memory", "high", "confirmed", "Low available memory", "Available memory is below the configured threshold; this alone does not prove an OOM kill.", []string{fmt.Sprintf("%.1f%% unavailable; %d bytes available", used, s.MemoryAvailable)}, []string{"Large working sets or memory limits"}, []string{"Inspect process RSS and memory PSI; review kernel OOM logs"}))
		}
	}
	if s.SwapTotal > 0 && float64(s.SwapFree)/float64(s.SwapTotal) < 0.1 {
		out = append(out, finding("memory.swap", "memory", "warning", "confirmed", "Swap nearly exhausted", "Swap capacity is nearly used; activity must be measured separately.", []string{fmt.Sprintf("%d/%d bytes swap free", s.SwapFree, s.SwapTotal)}, []string{"Historical or current memory demand"}, []string{"Check vmstat swap-in/swap-out before changing limits"}))
	}
	if s.CPUPercent >= c.CPUPercent {
		confidence := "confirmed"
		title := "High sampled CPU utilization"
		if len(s.Load) > 1 && s.Load[1] >= float64(s.CPUCount) {
			confidence = "probable"
			title = "Probable sustained CPU contention"
		}
		out = append(out, finding("cpu.high", "cpu", "warning", confidence, title, "CPU exceeded the threshold during the sample. Load also includes tasks in uninterruptible I/O.", []string{fmt.Sprintf("sample %.1f%%; load %v; CPUs %d", s.CPUPercent, s.Load, s.CPUCount)}, []string{"Compute load or I/O stalls"}, []string{"Sample repeatedly; inspect process CPU and I/O wait"}))
	}
	for _, d := range s.Disks {
		if d.Total > 0 && 100*(1-float64(d.Available)/float64(d.Total)) >= c.DiskPercent {
			out = append(out, finding("disk.capacity:"+d.Mount, "disk", "high", "confirmed", "Filesystem nearly full", "Space available to unprivileged users is below threshold.", []string{fmt.Sprintf("%s: %d/%d bytes available", d.Mount, d.Available, d.Total)}, []string{"Data growth or retained logs"}, []string{"Inspect filesystem usage and deleted open files; do not delete blindly"}))
		}
		if d.Inodes > 0 && float64(d.FreeInodes)/float64(d.Inodes) <= 0.05 {
			out = append(out, finding("disk.inodes:"+d.Mount, "disk", "high", "confirmed", "Inodes nearly exhausted", "New files may fail even when byte capacity remains.", []string{fmt.Sprintf("%s: %d/%d inodes free", d.Mount, d.FreeInodes, d.Inodes)}, []string{"Many small files"}, []string{"Identify directories with excessive file counts"}))
		}
	}
	return out
}
func service(o types.Observation, _ config.Config) []types.Finding {
	s, ok := o.Data.(types.Service)
	if !ok {
		return nil
	}
	out := []types.Finding{}
	if s.Active == "failed" {
		out = append(out, finding("service.failed:"+s.Name, "service", "high", "confirmed", "Service failed", "systemd reports failed state; the state alone does not establish a root cause.", []string{s.Name + ": " + s.Active + "/" + s.Sub + " result=" + s.Result}, []string{"Startup or runtime failure"}, []string{"Review journal entries, unit configuration, and exit status"}))
	}
	if s.Result == "oom-kill" {
		out = append(out, finding("service.oom:"+s.Name, "service", "high", "confirmed", "Service affected by OOM kill", "systemd recorded an OOM kill result.", []string{"Result=oom-kill"}, []string{"Host or cgroup memory exhaustion"}, []string{"Review kernel OOM records and MemoryMax"}))
	}
	for _, l := range s.Logs {
		low := strings.ToLower(l)
		if strings.Contains(low, "address already in use") || strings.Contains(low, "eaddrinuse") {
			out = append(out, finding("service.bind:"+s.Name, "network", "high", "probable", "Service reported a port conflict", "The journal contains a bind conflict; the message may be historical and is untrusted application output.", []string{l}, []string{"Another listener or duplicate bind configuration"}, []string{"Compare current listening sockets with the service listen configuration"}))
			break
		}
	}
	return out
}
func connectivity(o types.Observation, _ config.Config) []types.Finding {
	c, ok := o.Data.(types.Check)
	if !ok || c.Outcome == "connected" || c.Outcome == "resolved" {
		return nil
	}
	title, explanation := "Connectivity check failed", "The check failed; inspect the evidence."
	causes := []string{}
	steps := []string{"Repeat the check and compare DNS, routing and destination listener state"}
	switch c.Outcome {
	case "dns_failure":
		title = "DNS resolution failed"
		explanation = "The configured resolver could not resolve the requested host."
		causes = []string{"Resolver failure, unavailable name or DNS configuration"}
	case "refused":
		title = "TCP connection refused"
		explanation = "The peer or an intermediary actively rejected the connection."
		causes = []string{"No listener, active firewall reject or incorrect destination"}
	case "timeout":
		title = "TCP connection timed out"
		explanation = "No connection completed before the deadline. A firewall is only an unverified hypothesis."
		causes = []string{"Packet loss, destination unavailable, routing issue or possible packet filtering (unverified)"}
	case "unreachable":
		title = "Destination unreachable"
		explanation = "The network stack reported an unreachable route or host."
		causes = []string{"Missing route or unreachable gateway/host"}
	}
	return []types.Finding{finding("network."+c.Outcome, "network", "high", "confirmed", title, explanation, []string{fmt.Sprintf("%s:%d: %s", c.Host, c.Port, c.Detail)}, causes, steps)}
}
func container(o types.Observation, _ config.Config) []types.Finding {
	c, ok := o.Data.(types.Container)
	if !ok {
		return nil
	}
	out := []types.Finding{}
	add := func(id, title, confidence, detail string) {
		out = append(out, finding("container."+id+":"+c.ID, "container", "high", confidence, title, detail, []string{fmt.Sprintf("%s/%s state=%s health=%s exit=%d restarts=%d", c.Runtime, c.Name, c.Status, c.Health, c.ExitCode, c.Restarts)}, []string{"Application failure or resource configuration"}, []string{"Review logs, runtime events and resource limits"}))
	}
	if c.OOMKilled {
		add("oom", "Container was OOM killed", "confirmed", "The runtime recorded OOMKilled=true.")
	}
	if c.Health == "unhealthy" {
		add("health", "Container health check failed", "confirmed", "The runtime health check is unhealthy; inspect its output.")
	}
	if c.Restarts >= 3 {
		add("restarts", "Possible container restart loop", "probable", "The cumulative restart count is elevated; compare runtime events over time to confirm a current loop.")
	}
	if c.Status == "exited" && c.ExitCode != 0 {
		add("exit", "Container exited with error", "confirmed", "The runtime recorded a nonzero exit status.")
	}
	if c.MemoryLimit > 0 && c.MemoryUsage > c.MemoryLimit*9/10 {
		add("memory", "Container close to memory limit", "confirmed", "Reported usage exceeds 90% of the configured limit; cache accounting may affect usage.")
	}
	return out
}
