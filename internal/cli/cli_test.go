package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, tc := range []struct {
		args []string
		fail bool
		want string
	}{{[]string{"version"}, false, "ssync test"}, {[]string{"diagnose", "--pid", "-1"}, true, "pid"}, {[]string{"network", "check", "--host", "localhost", "--port", "70000"}, true, "port"}, {[]string{"services", "inspect"}, true, "arg"}, {[]string{"--format", "xml", "status"}, true, "format"}, {[]string{"containers", "inspect", "abc", "--runtime", "other"}, true, "runtime"}, {[]string{"help"}, false, "SystemSync"}, {[]string{"completion", "bash"}, false, "ssync"}, {[]string{"config", "show"}, false, "sample_interval"}} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			root := New(Build{Version: "test"})
			var b bytes.Buffer
			root.SetOut(&b)
			root.SetErr(&b)
			root.SetArgs(append([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")}, tc.args...))
			e := root.Execute()
			if (e != nil) != tc.fail {
				t.Fatalf("error=%v output=%s", e, b.String())
			}
			s := b.String()
			if e != nil {
				s += e.Error()
			}
			if !strings.Contains(s, tc.want) {
				t.Fatalf("missing %q: %s", tc.want, s)
			}
		})
	}
}
