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
		if status == "running" && job.cancel != nil {
			job.cancel()
		}
	}
}

func (c *coordinator) backgroundAgentsBusy(sessionID string) bool {
	for _, job := range c.registrySnapshot() {
		if sessionID != "" && job.sessionID != sessionID && job.parentSessionID != sessionID {
			continue
		}
		status, _ := job.snapshot()
		if status == "running" {
			return true
		}
	}
	return false
}
