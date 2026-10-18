package backend

import "github.com/SpherePrime/CLI/internal/agent"

func (b *Backend) QueueEntries(workspaceID, sessionID string) ([]agent.QueuedPrompt, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.AgentCoordinator == nil {
		return []agent.QueuedPrompt{}, nil
	}
	control, ok := ws.AgentCoordinator.(agent.QueueController)
	if !ok {
		return nil, agent.ErrQueueControlUnsupported
	}
	return control.QueueEntries(sessionID), nil
}

func (b *Backend) RunQueuedNow(workspaceID, sessionID, id string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.AgentCoordinator == nil {
		return ErrAgentNotInitialized
	}
	control, ok := ws.AgentCoordinator.(agent.QueueController)
	if !ok {
		return agent.ErrQueueControlUnsupported
	}
	ws.runMu.Lock()
	if ws.closing {
		ws.runMu.Unlock()
		return ErrWorkspaceClosing
	}
	ws.runWG.Add(1)
	ws.runMu.Unlock()
	done, err := control.RunQueuedNow(ws.ctx, sessionID, id)
	if err != nil {
		ws.runWG.Done()
		return err
	}
	go func() { defer ws.runWG.Done(); <-done }()
	return nil
}
