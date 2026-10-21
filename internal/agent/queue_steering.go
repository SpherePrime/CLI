package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const steeringInstruction = "The user supplied additional instructions while the current task was running. Treat these follow-up messages as additions to the unfinished task and continue the original work. If the user explicitly changes or cancels the task, follow that change. Preserve existing permission requirements; a follow-up does not grant extra permissions."

func hasSteeringCall(calls []SessionAgentCall) bool {
	for _, call := range calls {
		if call.steering {
			return true
		}
	}
	return false
}

func withSteeringInstruction(messages []fantasy.Message) []fantasy.Message {
	for _, entry := range messages {
		if entry.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range entry.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && text.Text == steeringInstruction {
				return messages
			}
		}
	}
	return append([]fantasy.Message{fantasy.NewSystemMessage(steeringInstruction)}, messages...)
}

func (a *sessionAgent) publishSteeringComplete(ctx context.Context, calls []SessionAgentCall, complete notify.RunComplete) {
	for _, call := range calls {
		if !call.steering || call.RunID == "" {
			continue
		}
		complete.SessionID = call.SessionID
		complete.RunID = call.RunID
		a.publishRunComplete(ctx, call, complete)
	}
}
