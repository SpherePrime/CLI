package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/pubsub"
)

func TestScopedChannelsDoNotLeakBetweenProjectsOrWithoutOptIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	project := t.TempDir()
	selected := SubscribeChannelEvents(ctx, project, []string{"scope-fixture"})
	other := SubscribeChannelEvents(ctx, t.TempDir(), []string{"scope-fixture"})
	disabled := SubscribeChannelEvents(ctx, project, nil)
	otherServer := SubscribeChannelEvents(ctx, project, []string{"different-server"})
	unowned := SubscribeChannelEvents(ctx, "", []string{"scope-fixture"})
	publishChannelMessage(ctx, "scope-fixture", json.RawMessage(`{"content":"build passed"}`), project)
	select {
	case event := <-selected:
		if event.Payload.ChannelID == "" || event.Payload.WorkspacePath == "" || event.Payload.ChannelReceivedAt.IsZero() {
			t.Fatal("channel identity missing")
		}
	case <-time.After(time.Second):
		t.Fatal("scoped channel was not delivered")
	}
	for _, events := range []<-chan pubsub.Event[Event]{other, disabled, otherServer, unowned} {
		select {
		case <-events:
			t.Fatal("channel leaked to another project or disabled subscriber")
		case <-time.After(30 * time.Millisecond):
		}
	}
}

func TestScopedChannelsRejectMessagesWithoutWorkspaceOwnership(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := SubscribeChannelEvents(ctx, t.TempDir(), []string{"unowned-fixture"})
	publishChannelMessage(ctx, "unowned-fixture", json.RawMessage(`{"content":"unowned"}`))
	select {
	case <-events:
		t.Fatal("unowned message reached a workspace")
	case <-time.After(30 * time.Millisecond):
	}
}
