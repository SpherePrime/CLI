package agent

func (c *coordinator) cachedSubAgents() []SessionAgent {
	c.subAgentMu.Lock()
	defer c.subAgentMu.Unlock()
	agents := make([]SessionAgent, 0, len(c.subAgents))
	for _, agent := range c.subAgents {
		agents = append(agents, agent)
	}
	return agents
}

func (c *coordinator) cancelBackgroundAgents(sessionID string) {
	for _, job := range c.registrySnapshot() {
		if sessionID != "" && job.sessionID != sessionID && job.parentSessionID != sessionID {
			continue
		}
		status, _ := job.snapshot()
		job.mu.Lock()
		cancel := job.cancel
		job.pending = nil
		job.seen = true
		job.mu.Unlock()
		if (status == "running" || status == "queued") && cancel != nil {
			cancel()
		}
	}
}

func (c *coordinator) backgroundAgentsBusy(sessionID string) bool {
	for _, job := range c.registrySnapshot() {
		if sessionID != "" && job.sessionID != sessionID && job.parentSessionID != sessionID {
			continue
		}
		status, _ := job.snapshot()
		if status == "running" || status == "queued" {
			return true
		}
	}
	return false
}
