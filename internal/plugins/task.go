package plugins

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/voice"
)

// TaskState is where an install task stands.
type TaskState uint8

const (
	// TaskRunning means the install is still downloading or installing.
	TaskRunning TaskState = iota
	// TaskDone means the install finished successfully.
	TaskDone
	// TaskFailed means the install stopped with an error.
	TaskFailed
)

// Snapshot is an immutable view of a task for rendering.
type Snapshot struct {
	State    TaskState
	Progress Progress
	Text     string
	Err      error
}

// Task tracks one plugin install running in the background. The plugins menu
// polls Snapshot on its spinner tick, so reopening the dialog mid-install
// keeps showing the same progress.
type Task struct {
	pluginName string
	ctx        context.Context
	cancel     context.CancelFunc
	finished   chan struct{}

	mu       sync.Mutex
	state    TaskState
	progress Progress
	text     string
	err      error
}

var activeTasks sync.Map // plugin name -> *Task

// installDeadline bounds one plugin install. Downloads of a few hundred MB
// plus an installer run fit well inside it.
const installDeadline = 30 * time.Minute

// InstallAsync starts plugin.Install in the background and returns its task.
// When an install for the plugin is already running, that task is returned
// unchanged.
func InstallAsync(plugin Plugin, settings voice.Settings) *Task {
	if running := Running(plugin.Name); running != nil {
		return running
	}
	ctx, cancel := context.WithTimeout(context.Background(), installDeadline)
	task := &Task{
		pluginName: plugin.Name,
		ctx:        ctx,
		cancel:     cancel,
		finished:   make(chan struct{}),
		state:      TaskRunning,
	}
	activeTasks.Store(plugin.Name, task)
	go task.run(plugin, settings)
	return task
}

// Running returns the in-flight install task for a plugin, or nil when no
// install is active.
func Running(name string) *Task {
	value, ok := activeTasks.Load(name)
	if !ok {
		return nil
	}
	task, _ := value.(*Task)
	if task == nil || task.Snapshot().State != TaskRunning {
		return nil
	}
	return task
}

// Name is the plugin this task installs.
func (t *Task) Name() string { return t.pluginName }

func (t *Task) run(plugin Plugin, settings voice.Settings) {
	defer t.cancel()
	defer activeTasks.Delete(t.pluginName)
	defer close(t.finished)

	err := plugin.Install(t.ctx, settings, t.report)
	state := TaskDone
	if err != nil && !errors.Is(err, context.Canceled) {
		state = TaskFailed
	}
	t.mu.Lock()
	t.state, t.err = state, err
	t.mu.Unlock()
}

// Snapshot copies the current state without holding the task open.
func (t *Task) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Snapshot{State: t.state, Progress: t.progress, Text: t.text, Err: t.err}
}

// Done closes when the task leaves the running state.
func (t *Task) Done() <-chan struct{} { return t.finished }

// Cancel stops a running install (e.g. the user switched the plugin off).
func (t *Task) Cancel() { t.cancel() }

func (t *Task) report(progress Progress) {
	t.mu.Lock()
	t.progress = progress
	if progress.Text != "" {
		t.text = progress.Text
	}
	t.mu.Unlock()
}
