package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const stoppedWorkersHeader = "Team workers of this chat were stopped before they finished:"

func stoppedWorker(status string) bool {
	return status == "canceled" || status == "interrupted"
}

func sortWorkers(workers []AgentWorker) {
	sort.SliceStable(workers, func(i, k int) bool {
		if workers[i].StartedAt != workers[k].StartedAt {
			return workers[i].StartedAt < workers[k].StartedAt
		}
		return workers[i].SessionID < workers[k].SessionID
	})
}

func stoppedWorkersNote(workers []AgentWorker) string {
	var lines []string
	for _, worker := range workers {
		if !stoppedWorker(worker.Status) {
			continue
		}
		task := strings.Join(strings.Fields(worker.Task), " ")
		if len([]rune(task)) > 200 {
			task = string([]rune(task)[:200]) + "…"
		}
		lines = append(lines, fmt.Sprintf("- %s (worker_id %s, %s): %s", worker.Name, worker.ID, worker.Agent, task))
	}
	if len(lines) == 0 {
		return ""
	}
	return stoppedWorkersHeader + "\n" + strings.Join(lines, "\n") +
		"\nWhen the user asks to continue or resume, continue these workers with agent_message op=send and their worker_id; they keep their previous conversation. Do not spawn new agents for the same work."
}

func (c *coordinator) teamNote(ctx context.Context, sessionID string) string {
	if c.cfg == nil || c.cfg.Config().Options.DisableMultiAgent {
		return ""
	}
	root, err := c.teamRoot(ctx, sessionID)
	if err != nil || root != sessionID {
		return ""
	}
	workers, err := c.AgentWorkers(ctx, sessionID)
	if err != nil {
		return ""
	}
	return stoppedWorkersNote(workers)
}

func withTeamNote(messages []fantasy.Message, note string) []fantasy.Message {
	if note == "" {
		return messages
	}
	for _, entry := range messages {
		if entry.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range entry.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.HasPrefix(text.Text, stoppedWorkersHeader) {
				return messages
			}
		}
	}
	return append([]fantasy.Message{fantasy.NewSystemMessage(note)}, messages...)
}
