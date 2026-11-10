package agent

import (
	"log/slog"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
)

var orphanReviewDelay = 3 * time.Second

const orphanReviewStart = "Every code agent of the team has finished. Review the work described in your assignment now."

func (c *coordinator) reviewerLinked(root string, reviewer *agentJob) bool {
	reviewer.mu.Lock()
	name, id, sessionID := reviewer.name, reviewer.id, reviewer.sessionID
	reviewer.mu.Unlock()
	for _, job := range c.registrySnapshot() {
		if job == reviewer || job.rootSessionID != root || job.agent != config.AgentCode {
			continue
		}
		job.mu.Lock()
		target := strings.TrimLeft(strings.TrimSpace(job.reviewer), "@#")
		job.mu.Unlock()
		if target != "" && (strings.EqualFold(target, name) || target == id || target == sessionID) {
			return true
		}
	}
	return false
}

func (c *coordinator) wakeOrphanReviewers(root string) {
	if root == "" || c.activeCodeAgents(root) > 0 {
		return
	}
	for _, job := range c.registrySnapshot() {
		if job.rootSessionID != root || job.agent != config.AgentReview {
			continue
		}
		job.mu.Lock()
		orphanCandidate := job.status == "waiting" && job.awaitHandoff
		job.mu.Unlock()
		if !orphanCandidate || c.reviewerLinked(root, job) {
			continue
		}
		if _, err := c.sendWorkerMessage(c.workerContext(), root, job.id, orphanReviewStart, nil, true); err != nil {
			slog.Warn("Starting unlinked reviewer failed", "job", job.id, "error", err)
		}
	}
}
