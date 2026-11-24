package shell

import (
	"context"
	"testing"
	"time"
)

func TestBackgroundJobRetainsSessionAndWorkspaceOwnership(t *testing.T) {
	manager := newBackgroundShellManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job, err := manager.Start(ctx, t.TempDir(), nil, "echo owned", "fixture", BackgroundOwnership{SessionID: "selected-session", WorkspacePath: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if job.SessionID != "selected-session" || job.WorkspacePath != "project" {
		t.Fatalf("wrong owner: %+v", job)
	}
	select {
	case <-job.done:
	case <-time.After(5 * time.Second):
		t.Fatal("fixture job did not finish")
	}
}

func TestBackgroundShellManager_KillByWorkspaceOnlyKillsMatchingWorkspace(t *testing.T) {
	manager := newBackgroundShellManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirA := t.TempDir()
	dirB := t.TempDir()
	jobA, err := manager.Start(ctx, dirA, nil, sleepCommand(10), "a", BackgroundOwnership{WorkspacePath: dirA})
	if err != nil {
		t.Fatal(err)
	}
	jobB, err := manager.Start(ctx, dirB, nil, sleepCommand(10), "b", BackgroundOwnership{WorkspacePath: dirB})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		killCtx, killCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer killCancel()
		manager.KillAll(killCtx)
	})

	killCtx, killCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer killCancel()
	manager.KillByWorkspace(killCtx, dirA)

	if _, ok := manager.Get(jobA.ID); ok {
		t.Fatal("expected workspace A job to be removed")
	}
	if _, ok := manager.Get(jobB.ID); !ok {
		t.Fatal("expected workspace B job to remain tracked")
	}
	if jobB.IsDone() {
		t.Fatal("expected workspace B job to remain running")
	}
}
