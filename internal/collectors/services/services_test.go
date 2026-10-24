package services

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	output string
	err    error
	args   []string
}

func (f *fakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.args = args
	return []byte(f.output), f.err
}
func TestInspect(t *testing.T) {
	f := &fakeRunner{output: "LoadState=loaded\nActiveState=failed\nSubState=failed\nResult=oom-kill\nMainPID=0\nNRestarts=3\nExecMainCode=2\nExecMainStatus=9\n"}
	s, e := (Adapter{f}).Inspect(context.Background(), "nginx.service")
	if e != nil || s.Result != "oom-kill" || s.Restarts != 3 {
		t.Fatalf("%+v %v", s, e)
	}
	if !strings.Contains(strings.Join(f.args, " "), "-- nginx.service") {
		t.Fatalf("%v", f.args)
	}
}
func TestUnavailableAndNames(t *testing.T) {
	if _, e := (Adapter{&fakeRunner{err: errors.New("permission denied")}}).Inspect(context.Background(), "nginx"); e == nil {
		t.Fatal("ignored permission error")
	}
	for _, name := range []string{"", "--all", "*.service", "foo/bar", "foo\nbar"} {
		if ValidateName(name) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if _, e := Parse("missing", "LoadState=not-found"); e == nil {
		t.Fatal("ignored missing unit")
	}
}
func TestLogs(t *testing.T) {
	a := Adapter{&fakeRunner{output: "{\"MESSAGE\":\"failed bind\"}\n{\"MESSAGE\":\"next\"}\n"}}
	logs, e := a.Logs(context.Background(), "nginx", 2)
	if e != nil || len(logs) != 2 {
		t.Fatalf("%v %v", logs, e)
	}
}
