package backend

import (
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/proto"
)

func TestCloseInProcessClearsHoldsAndStopsTeardown(t *testing.T) {
	b, _ := newTestBackend(t)
	workspace, shutdowns := insertTestWorkspace(t, b, t.TempDir())
	client := &clientState{holdTimer: time.NewTimer(time.Hour)}
	workspace.clients["client"] = client
	b.shutdownTimer = time.NewTimer(time.Hour)
	b.CloseInProcess()
	b.CloseInProcess()
	b.expireHold(workspace, "client", client)
	b.teardown(workspace)
	if count := shutdowns.Load(); count != 1 {
		t.Fatalf("workspace shutdown count: %d", count)
	}
	if !b.closing || b.shutdownTimer != nil || client.holdTimer != nil {
		t.Fatal("lifecycle timers remain active")
	}
	if len(b.ListWorkspaces()) != 0 || len(b.pathIndex) != 0 || len(workspace.clients) != 0 {
		t.Fatal("workspace state remains registered")
	}
	if _, _, err := b.CreateWorkspace(proto.Workspace{Path: workspace.Path, ClientID: newClientID(t)}); err == nil {
		t.Fatal("closed backend accepted workspace")
	}
}
