package diagnostics

import (
	"context"
	"fmt"
	"github.com/manitrix/systemsync/internal/collectors/containers"
	"github.com/manitrix/systemsync/internal/collectors/network"
	"github.com/manitrix/systemsync/internal/collectors/process"
	"github.com/manitrix/systemsync/internal/collectors/services"
	"github.com/manitrix/systemsync/internal/collectors/system"
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/internal/rules"
	"github.com/manitrix/systemsync/pkg/types"
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"time"
)

type Task struct {
	Name    string
	Collect func(context.Context) (any, error)
}

func Run(ctx context.Context, c config.Config, tasks []Task) types.Report {
	r := types.Report{SchemaVersion: "1", Timestamp: time.Now().UTC(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Observations: make([]types.Observation, len(tasks)), Findings: []types.Finding{}}
	sem := make(chan struct{}, c.Concurrency)
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(i int, t Task) {
			defer wg.Done()
			o := types.Observation{Source: t.Name, Timestamp: time.Now().UTC()}
			defer func() { r.Observations[i] = o }()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				o.Error = ctx.Err().Error()
				return
			}
			local, cancel := context.WithTimeout(ctx, c.Timeout)
			defer cancel()
			data, err := t.Collect(local)
			o.Data = data
			if err != nil {
				o.Error = err.Error()
			}
		}(i, t)
	}
	wg.Wait()
	r.Findings = rules.Default().Evaluate(r.Observations, c)
	r.Findings = rules.Correlate(r.Findings, r.Observations)
	return r
}
func Tasks(c config.Config, pid int, service string) []Task {
	tasks := []Task{}
	add := func(name string, f func(context.Context) (any, error)) {
		if !slices.Contains(c.Disabled, name) {
			tasks = append(tasks, Task{Name: name, Collect: f})
		}
	}
	add("system", func(ctx context.Context) (any, error) { return system.Collect(ctx, c.SampleInterval) })
	add("process", func(ctx context.Context) (any, error) { return process.Collect(ctx, pid, c.SampleInterval) })
	add("network", func(ctx context.Context) (any, error) { return network.Collect(ctx) })
	if service != "" {
		add("services", func(ctx context.Context) (any, error) {
			a := services.New()
			s, e := a.Inspect(ctx, service)
			if e != nil {
				return nil, e
			}
			logs, e := a.Logs(ctx, service, c.LogLimit)
			s.Logs = logs
			return s, e
		})
	} else {
		add("services", func(ctx context.Context) (any, error) { return services.New().List(ctx) })
	}
	for _, adapter := range containers.Available(c.LogLimit) {
		adapter := adapter
		if !slices.Contains(c.Disabled, "containers") {
			tasks = append(tasks, Task{Name: "containers." + adapter.Name(), Collect: func(ctx context.Context) (any, error) { return adapter.List(ctx) }})
		}
	}
	return tasks
}
func Observe(ctx context.Context, c config.Config, name string, f func(context.Context) (any, error)) types.Report {
	return Run(ctx, c, []Task{{Name: name, Collect: f}})
}
func DebugReport(logger *slog.Logger, r types.Report) {
	for _, o := range r.Observations {
		if o.Error != "" {
			logger.Debug("collector unavailable", "source", o.Source, "error", o.Error)
		}
	}
}
func Summary(r types.Report) string {
	return fmt.Sprintf("%d observations, %d findings", len(r.Observations), len(r.Findings))
}
