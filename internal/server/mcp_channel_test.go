package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
)

func TestScopedChannelHasDedicatedEnvelope(t *testing.T) {
	message := mcp.Event{Type: mcp.EventChannelMessage, Name: "inbox", WorkspacePath: "project", ChannelID: "event", ChannelReceivedAt: time.Now().UTC(), ChannelMessage: `<channel source="inbox">hello</channel>`}
	envelope := wrapEvent(pubsub.Event[mcp.Event]{Type: pubsub.CreatedEvent, Payload: message})
	if envelope == nil || envelope.Type != "mcp_channel_message" {
		t.Fatalf("invalid channel envelope: %+v", envelope)
	}
	var decoded pubsub.Event[proto.MCPChannelMessage]
	if err := json.Unmarshal(envelope.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Payload.ID != "event" || decoded.Payload.WorkspacePath != "project" || decoded.Payload.Content != message.ChannelMessage {
		t.Fatalf("channel payload lost: %+v", decoded)
	}
}
