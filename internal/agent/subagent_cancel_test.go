package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCoordinatorCancelStopsBackgroundChildrenOnlyForSelectedSession(t *testing.T) {
	coord := newJobCoordinator(t, testEnv(t))
	coord.mainAgent = &mockSessionAgent{}
	selectedCtx, selectedCancel := context.WithCancel(t.Context())
	otherCtx, otherCancel := context.WithCancel(t.Context())
	t.Cleanup(selectedCancel)
	t.Cleanup(otherCancel)
	coord.agentJobs["selected"] = &agentJob{sessionID: "child", parentSessionID: "parent", status: "running", cancel: selectedCancel}
	coord.agentJobs["other"] = &agentJob{sessionID: "other-child", parentSessionID: "other", status: "running", cancel: otherCancel}
	require.True(t, coord.IsSessionBusy("parent"))
	require.True(t, coord.IsBusy())
	coord.Cancel("parent")
	require.ErrorIs(t, selectedCtx.Err(), context.Canceled)
	require.NoError(t, otherCtx.Err())
	coord.Cancel("other-child")
	require.ErrorIs(t, otherCtx.Err(), context.Canceled)
	coord.agentJobs["selected"].status = "canceled"
	coord.agentJobs["other"].status = "done"
	require.False(t, coord.IsBusy())
	require.False(t, coord.IsSessionBusy("parent"))
}

func TestCoordinatorCancelReachesSelectedTaskAgent(t *testing.T) {
	worker := &mockSessionAgent{}
	coord := &coordinator{mainAgent: &mockSessionAgent{}, subAgents: map[string]SessionAgent{"task": worker}}
	coord.Cancel("task-session")
	require.Equal(t, []string{"task-session"}, worker.cancelled)
}
