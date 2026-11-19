package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type gameDevCapturingCoordinator struct {
	blockingCoordinator
	mu     sync.Mutex
	gotVal bool
	gotOk  bool
	doneCh chan struct{}
}

func (c *gameDevCapturingCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	val, ok := agent.GameDevFromContext(ctx)
	c.mu.Lock()
	c.gotVal = val
	c.gotOk = ok
	c.mu.Unlock()
	close(c.doneCh)
	return nil, nil
}

func TestSendMessageGameDevIsPerRunAndDoesNotMutateConfig(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	b, _ := newTestBackend(t)
	coord := &gameDevCapturingCoordinator{
		blockingCoordinator: *newBlockingCoordinator(),
		doneCh:              make(chan struct{}),
	}
	ws := insertOrchestratorWorkspace(t, b, &forgetTrackingCoordinator{})
	ws.App.SetCoordinator(coord)

	ws.Cfg.OverrideGameDev(false)
	gdTrue := true

	err := b.SendMessage(ws.ID, proto.AgentMessage{
		SessionID: "chat-1",
		Prompt:    "build a game",
		GameDev:   &gdTrue,
	})
	require.NoError(t, err)

	select {
	case <-coord.doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for RunAccepted")
	}
	ws.runWG.Wait()

	coord.mu.Lock()
	gotVal := coord.gotVal
	gotOk := coord.gotOk
	coord.mu.Unlock()

	require.True(t, gotOk)
	require.True(t, gotVal)
	require.NotNil(t, ws.Cfg.Overrides().GameDev)
	require.False(t, *ws.Cfg.Overrides().GameDev)
	require.NotContains(t, ws.Cfg.Config().SessionGameDev, "chat-1")
	require.False(t, ws.Cfg.IsGameDevEnabled("chat-1"))
}
