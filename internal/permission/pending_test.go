package permission

import (
	"context"
	"testing"
	"time"
)

func TestPendingSnapshotClearsOnCancellation(t *testing.T) {
	s := NewPermissionService(t.TempDir(), false, nil).(*permissionService)
	ctx, cancel := context.WithCancel(context.Background())
	events := s.Subscribe(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Request(ctx, CreatePermissionRequest{SessionID: "s", ToolCallID: "tool", ToolName: "bash"})
	}()
	select {
	case ev := <-events:
		pending := s.Pending()
		if len(pending) != 1 || pending[0].ID != ev.Payload.ID {
			t.Fatalf("missing pending permission: %+v", pending)
		}
	case <-time.After(time.Second):
		t.Fatal("permission not published")
	}
	cancel()
	<-done
	if len(s.Pending()) != 0 {
		t.Fatal("cancelled permission remains pending")
	}
}
