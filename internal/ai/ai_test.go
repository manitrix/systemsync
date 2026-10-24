package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/pkg/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedaction(t *testing.T) {
	for _, s := range []string{"password=hunter2", "Authorization: Bearer abc.def.ghi", "sk-abcdefghijklmnopqrst", "-----BEGIN RSA PRIVATE KEY-----\nsecret\n-----END RSA PRIVATE KEY-----", "http://user:pass@example.com"} {
		v := Redact(s)
		if v == s || strings.Contains(v, "hunter2") || strings.Contains(v, "abc.def") {
			t.Errorf("failed redact %q => %q", s, v)
		}
	}
}
func TestProjection(t *testing.T) {
	r := types.Report{SchemaVersion: "1", Observations: []types.Observation{{Source: "private", Data: "SECRET_RAW_LOG"}}, Findings: []types.Finding{{ID: "secret-name", Evidence: []string{"SECRET_EVIDENCE"}, Title: "test", Explanation: "pressure"}}}
	b, e := Payload(r)
	if e != nil || !json.Valid(b) || strings.Contains(string(b), "SECRET") || strings.Contains(string(b), "secret-name") {
		t.Fatalf("%s %v", b, e)
	}
}
func TestConsent(t *testing.T) {
	c := config.Default().AI
	c.Endpoint = "https://example.com/v1"
	if ValidateEndpoint(c) == nil {
		t.Fatal("external transmission without consent")
	}
	c.Consent = true
	if e := ValidateEndpoint(c); e != nil {
		t.Fatal(e)
	}
	c.Endpoint = "http://example.com"
	if ValidateEndpoint(c) == nil {
		t.Fatal("insecure external")
	}
	c.Endpoint = "https://user:pass@example.com"
	if ValidateEndpoint(c) == nil {
		t.Fatal("embedded credentials")
	}
}
func TestProviders(t *testing.T) {
	for _, kind := range []string{"ollama", "openai"} {
		t.Run(kind, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Error("wrong method")
				}
				if kind == "ollama" {
					fmt.Fprint(w, `{"message":{"content":"review memory"}}`)
				} else {
					fmt.Fprint(w, `{"choices":[{"message":{"content":"review memory"}}]}`)
				}
			}))
			defer s.Close()
			c := config.Default().AI
			c.Provider = kind
			c.Endpoint = s.URL
			p := New(c)
			v, e := p.Explain(context.Background(), types.Report{SchemaVersion: "1"})
			if e != nil || !strings.Contains(v, "review memory") {
				t.Fatalf("%s %v", v, e)
			}
		})
	}
}
func TestFailureAndDeadline(t *testing.T) {
	for _, slow := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slow {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(200 * time.Millisecond):
				}
			}
			w.WriteHeader(503)
		}))
		c := config.Default().AI
		c.Endpoint = s.URL
		p := New(c)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, e := p.Explain(ctx, types.Report{})
		cancel()
		s.Close()
		if e == nil {
			t.Fatal("ignored provider failure")
		}
	}
}

func TestRedactedPayloadStaysValidJSON(t *testing.T) {
	r := types.Report{Findings: []types.Finding{{Title: "credential", Explanation: "password=hunter2", Steps: []string{"token=secret-value"}}}}
	b, e := Payload(r)
	if e != nil || !json.Valid(b) || strings.Contains(string(b), "hunter2") || strings.Contains(string(b), "secret-value") {
		t.Fatalf("%s %v", b, e)
	}
}
