package plugins

import (
	"context"
	"github.com/SpherePrime/CLI/internal/voice"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentInstallReusesTask(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	defer close(release)
	plugin := Plugin{Name: t.TempDir(), Install: func(ctx context.Context, settings voice.Settings, report func(Progress)) error {
		calls.Add(1)
		<-release
		return nil
	}}
	start := make(chan struct{})
	tasks := make(chan *Task, 64)
	var callers sync.WaitGroup
	for range 64 {
		callers.Go(func() { <-start; tasks <- InstallAsync(plugin, voice.Settings{}) })
	}
	close(start)
	callers.Wait()
	close(tasks)
	var first *Task
	for task := range tasks {
		if first == nil {
			first = task
		}
		if task != first {
			t.Error("concurrent install created a different task")
		}
	}
	deadline := time.After(time.Second)
	for calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("install never started")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("install started %d times", calls.Load())
	}
}
