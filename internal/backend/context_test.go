package backend

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// contextCoordinator answers the ContextReporter interface so
// GetContextBreakdown can be exercised against a prepared workspace.
type contextCoordinator struct {
	blockingCoordinator
	breakdown agent.ContextBreakdown
	err       error
}

func (c *contextCoordinator) ContextBreakdown(ctx context.Context, sessionID string) (agent.ContextBreakdown, error) {
	return c.breakdown, c.err
}

func TestGetContextBreakdown_AgentNotInitializedWithoutConfig(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, nil)

	_, err := b.GetContextBreakdown(context.Background(), ws.ID, "session")
	require.ErrorIs(t, err, ErrAgentNotInitialized)
}

func TestGetContextBreakdown_ReturnsReporterResult(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws := insertAgentWorkspace(t, b, &contextCoordinator{breakdown: agent.ContextBreakdown{
		Categories: []agent.ContextCategory{
			{Key: agent.ContextSystemPrompt, Tokens: 1200, Items: 1},
			{Key: agent.ContextUserMessages, Tokens: 300, Items: 2},
		},
		EstimatedTotal: 1500,
		ReportedTotal:  1800,
		ContextWindow:  100000,
	}})

	breakdown, err := b.GetContextBreakdown(context.Background(), ws.ID, "session")
	require.NoError(t, err)
	require.Len(t, breakdown.Categories, 2)
	require.EqualValues(t, 1800, breakdown.ReportedTotal)
	require.EqualValues(t, 100000, breakdown.ContextWindow)
}

func TestGetContextBreakdown_WorkspaceNotFound(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	_, err := b.GetContextBreakdown(context.Background(), "missing", "session")
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}

var _ = proto.ContextBreakdown{}
