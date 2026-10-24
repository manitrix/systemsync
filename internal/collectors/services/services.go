package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"os/exec"
	"strconv"
	"strings"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	b, e := exec.CommandContext(ctx, name, args...).Output()
	if e != nil {
		if x, ok := e.(*exec.ExitError); ok {
			return b, fmt.Errorf("%s: %w: %s", name, e, strings.TrimSpace(string(x.Stderr)))
		}
		return b, e
	}
	return b, nil
}

type Adapter struct{ Runner Runner }

func New() Adapter { return Adapter{Runner: ExecRunner{}} }
func ValidateName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "/\x00\n\r") || strings.ContainsAny(name, "*?[]") {
		return fmt.Errorf("invalid exact service name %q", name)
	}
	return nil
}
func (a Adapter) List(ctx context.Context) ([]types.Service, error) {
	b, e := a.Runner.Run(ctx, "systemctl", "list-units", "--type=service", "--all", "--output=json", "--no-pager")
	if e != nil {
		return nil, e
	}
	var rows []struct{ Unit, Load, Active, Sub string }
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("systemctl JSON (systemd 245+ required): %w", err)
	}
	out := make([]types.Service, 0, len(rows))
	for _, r := range rows {
		out = append(out, types.Service{Name: r.Unit, Load: r.Load, Active: r.Active, Sub: r.Sub})
	}
	return out, nil
}
func Parse(name, s string) (types.Service, error) {
	v := types.Service{Name: name}
	for _, line := range strings.Split(s, "\n") {
		k, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		var e error
		switch k {
		case "LoadState":
			v.Load = val
		case "ActiveState":
			v.Active = val
		case "SubState":
			v.Sub = val
		case "Result":
			v.Result = val
		case "MainPID":
			v.PID, e = strconv.Atoi(val)
		case "NRestarts":
			v.Restarts, e = strconv.Atoi(val)
		case "ExecMainCode":
			v.ExecCode, e = strconv.Atoi(val)
		case "ExecMainStatus":
			v.ExecStatus, e = strconv.Atoi(val)
		}
		if e != nil {
			return v, e
		}
	}
	if v.Load == "" {
		return v, fmt.Errorf("missing systemd LoadState")
	}
	if v.Load == "not-found" {
		return v, fmt.Errorf("service %s not found", name)
	}
	return v, nil
}
func (a Adapter) Inspect(ctx context.Context, name string) (types.Service, error) {
	if e := ValidateName(name); e != nil {
		return types.Service{}, e
	}
	b, e := a.Runner.Run(ctx, "systemctl", "show", "--no-pager", "--property=LoadState,ActiveState,SubState,Result,MainPID,NRestarts,ExecMainCode,ExecMainStatus", "--", name)
	if e != nil {
		return types.Service{}, e
	}
	return Parse(name, string(b))
}
func (a Adapter) Logs(ctx context.Context, name string, limit int) ([]string, error) {
	if e := ValidateName(name); e != nil {
		return nil, e
	}
	b, e := a.Runner.Run(ctx, "journalctl", "--unit="+name, "--lines="+strconv.Itoa(limit), "--output=json", "--no-pager")
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var v struct {
			Message any `json:"MESSAGE"`
		}
		if e := json.Unmarshal([]byte(line), &v); e != nil {
			return nil, e
		}
		if msg, ok := v.Message.(string); ok {
			out = append(out, msg)
		}
	}
	return out, nil
}
