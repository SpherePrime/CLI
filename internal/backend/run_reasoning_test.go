package backend

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSendMessageRejectsInvalidReasoningBeforeAcceptance(t *testing.T) {
	b, _ := newTestBackend(t)
	coordinator := newBlockingCoordinator()
	ws := insertAgentWorkspace(t, b, coordinator)
	err := b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "session", Prompt: "hello", RunID: "run", ReasoningMode: "invalid"})
	require.ErrorContains(t, err, "invalid reasoning mode")
	require.Empty(t, ws.runs)
	require.Equal(t, int32(0), coordinator.runCount.Load())
}
