package backend

func (b *Backend) CloseInProcess() {
	b.mu.Lock()
	b.closing = true
	b.cancelShutdownLocked()
	workspaces := make([]*Workspace, 0)
	for id, workspace := range b.workspaces.Seq2() {
		workspace.clientsMu.Lock()
		for _, client := range workspace.clients {
			if client.holdTimer != nil {
				client.holdTimer.Stop()
				client.holdTimer = nil
			}
		}
		clear(workspace.clients)
		workspace.clientsMu.Unlock()
		b.workspaces.Del(id)
		workspaces = append(workspaces, workspace)
	}
	clear(b.pathIndex)
	b.mu.Unlock()
	for _, workspace := range workspaces {
		workspace.invokeShutdown()
	}
}
