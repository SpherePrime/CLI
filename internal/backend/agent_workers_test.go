package backend

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/app"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type workerCoordinatorStub struct {
	agent.Coordinator
	workers []agent.AgentWorker
	source  string
	target  string
	text    string
}

func (s *workerCoordinatorStub) AgentWorkers(context.Context, string) ([]agent.AgentWorker, error) {
	return s.workers, nil
}

func (s *workerCoordinatorStub) SendAgentWorkerMessage(_ context.Context, source, target, text string) (agent.AgentWorker, error) {
	s.source, s.target, s.text = source, target, text
	return s.workers[0], nil
}

func (s *workerCoordinatorStub) StopAgentWorker(_ context.Context, source, target string) error {
	s.source, s.target = source, target
	return nil
}

func TestAgentWorkerOperationsRequireTeamMembership(t *testing.T) {
	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, t.TempDir())
	stub := &workerCoordinatorStub{workers: []agent.AgentWorker{{ID: "worker", SessionID: "child", RootSessionID: "root"}}}
	ws.App = &app.App{AgentCoordinator: stub}
	_, err := b.ListAgentWorkerMessages(context.Background(), ws.ID, "root", "other-team")
	require.ErrorIs(t, err, ErrAgentWorkerNotFound)
	_, err = b.SendAgentWorkerMessage(context.Background(), ws.ID, "root", "other-team", "hello")
	require.ErrorIs(t, err, ErrAgentWorkerNotFound)
	require.Empty(t, stub.target)
	require.NoError(t, b.StopAgentWorker(context.Background(), ws.ID, "root", "worker"))
	require.Equal(t, "root", stub.source)
	require.Equal(t, "worker", stub.target)
}

func TestAgentWorkerSendValidatesTextAndNormalizesRoster(t *testing.T) {
	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, t.TempDir())
	stub := &workerCoordinatorStub{}
	ws.App = &app.App{AgentCoordinator: stub}
	workers, err := b.GetAgentWorkers(context.Background(), ws.ID, "root")
	require.NoError(t, err)
	require.NotNil(t, workers)
	stub.workers = []agent.AgentWorker{{ID: "worker", SessionID: "child", RootSessionID: "root"}}
	_, err = b.SendAgentWorkerMessage(context.Background(), ws.ID, "root", "worker", "  ")
	require.Error(t, err)
	_, err = b.SendAgentWorkerMessage(context.Background(), ws.ID, "root", "worker", "follow up")
	require.NoError(t, err)
	require.Equal(t, "follow up", stub.text)
	_, err = b.GetAgentWorkers(context.Background(), ws.ID, "")
	require.Error(t, err)
}
