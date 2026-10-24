package diagnostics

import (
	"context"
	"errors"
	"github.com/manitrix/systemsync/internal/config"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyAndErrors(t *testing.T) {
	c := config.Default()
	c.Concurrency = 2
	var active, max atomic.Int32
	tasks := []Task{}
	for i := 0; i < 8; i++ {
		tasks = append(tasks, Task{Name: "fixture", Collect: func(ctx context.Context) (any, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for {
				old := max.Load()
				if old >= n || max.CompareAndSwap(old, n) {
					break
				}
			}
			select {
			case <-time.After(5 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return "partial", errors.New("permission denied")
		}})
	}
	r := Run(context.Background(), c, tasks)
	if max.Load() > 2 || len(r.Observations) != 8 {
		t.Fatal("concurrency bound")
	}
	for _, o := range r.Observations {
		if o.Error != "permission denied" || o.Data != "partial" {
			t.Fatalf("%+v", o)
		}
	}
}
func TestCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := Run(ctx, config.Default(), []Task{{Name: "cancel", Collect: func(ctx context.Context) (any, error) { return nil, ctx.Err() }}})
	if r.Observations[0].Error == "" {
		t.Fatal("cancellation lost")
	}
}
