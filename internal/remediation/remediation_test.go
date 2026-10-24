package remediation

import (
	"context"
	"testing"
)

func TestApprovalAndAllowlist(t *testing.T) {
	p, e := DNSPlan("localhost")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Execute(context.Background(), p, false); e == nil {
		t.Fatal("executed without approval")
	}
	p.Operation = "shell"
	if _, e = Execute(context.Background(), p, true); e == nil {
		t.Fatal("allowed arbitrary action")
	}
	for _, s := range []string{"", "127.0.0.1", "foo;rm -rf /", "x\ny"} {
		if _, e := DNSPlan(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
