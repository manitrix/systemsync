// Package remediation enforces plan approval and an explicit operation allowlist.
// The initial release permits only a read-only DNS verification action.
package remediation

import (
	"context"
	"fmt"
	"net"
	"time"
)

type Plan struct {
	Operation   string `json:"operation"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Rollback    string `json:"rollback"`
}
type Result struct {
	Plan      Plan     `json:"plan"`
	Verified  bool     `json:"verified"`
	Addresses []string `json:"addresses"`
}

func DNSPlan(host string) (Plan, error) {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil {
		return Plan{}, fmt.Errorf("DNS verification requires a hostname")
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return Plan{}, fmt.Errorf("invalid DNS hostname")
		}
	}
	return Plan{Operation: "dns-recheck", Target: host, Description: "Resolve this single hostname using the system resolver; no system configuration changes", Rollback: "No state is modified; there is nothing to roll back"}, nil
}
func Execute(ctx context.Context, p Plan, approved bool) (Result, error) {
	result := Result{Plan: p}
	if !approved {
		return result, fmt.Errorf("explicit --approve is required after reviewing the plan")
	}
	expected, e := DNSPlan(p.Target)
	if e != nil {
		return result, e
	}
	if p != expected {
		return result, fmt.Errorf("plan is not an allowlisted operation")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupHost(ctx, p.Target)
	if e != nil {
		return result, e
	}
	result.Addresses = ips
	result.Verified = len(ips) > 0
	return result, nil
}
