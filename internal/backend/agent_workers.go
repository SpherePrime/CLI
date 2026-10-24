package backend

import (
	"context"
	"errors"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/message"
)

var ErrAgentWorkerNotFound = errors.New("agent worker not found in this team")
var ErrAgentWorkersUnsupported = errors.New("agent orchestration unavailable")
var ErrAgentWorkerInput = errors.New("session and nonempty message are required")

func (b *Backend) workerControl(workspaceID, sessionID string) (agent.AgentOrchestrator, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrAgentWorkerInput
	}
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.App == nil || ws.AgentCoordinator == nil {
		return nil, ErrAgentNotInitialized
	}
	control, ok := ws.AgentCoordinator.(agent.AgentOrchestrator)
	if !ok {
		return nil, ErrAgentWorkersUnsupported
	}
	return control, nil
}

func (b *Backend) GetAgentWorkers(ctx context.Context, workspaceID, sessionID string) ([]agent.AgentWorker, error) {
	control, err := b.workerControl(workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	workers, err := control.AgentWorkers(ctx, sessionID)
	if workers == nil && err == nil {
		workers = []agent.AgentWorker{}
	}
	return workers, err
}

func (b *Backend) findAgentWorker(ctx context.Context, workspaceID, sessionID, workerID string) (agent.AgentWorker, error) {
	workers, err := b.GetAgentWorkers(ctx, workspaceID, sessionID)
	if err != nil {
		return agent.AgentWorker{}, err
	}
	for _, worker := range workers {
		if worker.ID == workerID {
			return worker, nil
		}
	}
	return agent.AgentWorker{}, ErrAgentWorkerNotFound
}

func (b *Backend) ListAgentWorkerMessages(ctx context.Context, workspaceID, sessionID, workerID string) ([]message.Message, error) {
	worker, err := b.findAgentWorker(ctx, workspaceID, sessionID, workerID)
	if err != nil {
		return nil, err
	}
	return b.ListSessionMessages(ctx, workspaceID, worker.SessionID)
}

func (b *Backend) SendAgentWorkerMessage(ctx context.Context, workspaceID, sessionID, workerID, text string) (agent.AgentWorker, error) {
	if strings.TrimSpace(text) == "" || len(text) > 131072 {
		return agent.AgentWorker{}, ErrAgentWorkerInput
	}
	if _, err := b.findAgentWorker(ctx, workspaceID, sessionID, workerID); err != nil {
		return agent.AgentWorker{}, err
	}
	control, err := b.workerControl(workspaceID, sessionID)
	if err != nil {
		return agent.AgentWorker{}, err
	}
	return control.SendAgentWorkerMessage(ctx, sessionID, workerID, text)
}

func (b *Backend) StopAgentWorker(ctx context.Context, workspaceID, sessionID, workerID string) error {
	if _, err := b.findAgentWorker(ctx, workspaceID, sessionID, workerID); err != nil {
		return err
	}
	control, err := b.workerControl(workspaceID, sessionID)
	if err != nil {
		return err
	}
	return control.StopAgentWorker(ctx, sessionID, workerID)
}
