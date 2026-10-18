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
