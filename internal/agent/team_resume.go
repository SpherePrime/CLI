package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	stoppedWorkersHeader = "Team workers of this chat were stopped before they finished:"
	teamRosterHeader     = "Existing team workers in this chat:"
)

func stoppedWorker(status string) bool {
	return status == "canceled" || status == "interrupted"
}

func truncateWorkerTask(task string) string {
	clean := strings.Join(strings.Fields(task), " ")
	if len([]rune(clean)) > 200 {
		return string([]rune(clean)[:200]) + "…"
	}
	return clean
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
		task := truncateWorkerTask(worker.Task)
		lines = append(lines, fmt.Sprintf("- %s (worker_id %s, %s): %s", worker.Name, worker.ID, worker.Agent, task))
	}
	if len(lines) == 0 {
		return ""
	}
	return stoppedWorkersHeader + "\n" + strings.Join(lines, "\n") +
		"\nWhen the user asks to continue or resume, continue these workers with agent_message op=send and their worker_id; they keep their previous conversation. Do not spawn new agents for the same work."
}

func teamRosterNote(workers []AgentWorker) string {
	var lines []string
	for _, worker := range workers {
		label := worker.Name
		if label == "" {
			label = worker.Agent
		}
		meta := fmt.Sprintf("worker_id %s, role %s, status %s", worker.ID, worker.Agent, worker.Status)
		if worker.Reviewer != "" {
			meta += fmt.Sprintf(", reviewer %s", worker.Reviewer)
		}
		line := fmt.Sprintf("- %s (%s)", label, meta)
		if task := truncateWorkerTask(worker.Task); task != "" {
			line += ": " + task
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return teamRosterHeader + "\n" + strings.Join(lines, "\n") +
		"\nReuse rules:" +
		"\n- If a user request or follow-up relates to the same part or feature an existing worker already handled (for example modifying, fixing, or continuing something that worker built or explored), reuse that existing worker via agent_message (op=send, worker_id=<worker_id>). It retains its full conversation and context, and any linked review worker is automatically re-armed for a fresh review cycle. Do NOT spawn new -v2 or -fix agents for the same part." +
		"\n- Spawn new agents via agent only for genuinely new, independent parts not covered by existing workers."
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
	return teamRosterNote(workers)
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
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && (strings.HasPrefix(text.Text, stoppedWorkersHeader) || strings.HasPrefix(text.Text, teamRosterHeader)) {
				return messages
			}
		}
	}
	return append([]fantasy.Message{fantasy.NewSystemMessage(note)}, messages...)
}
