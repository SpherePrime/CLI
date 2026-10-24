package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const AgentMessageToolName = "agent_message"

type agentMessageParams struct {
	Op       string `json:"op,omitempty" description:"list (team roster) or send"`
	WorkerID string `json:"worker_id,omitempty" description:"Worker ID or session ID; use parent to send a progress report or request another assignment from the primary chat"`
	Text     string `json:"text,omitempty" description:"Self-contained message, findings, or follow-up assignment"`
}

func (c *coordinator) agentMessageTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(AgentMessageToolName, "List your team or send messages to a peer or parent. Messages stay within this primary chat. Busy workers receive follow-ups after their current turn; idle workers continue their existing conversation. Request further assignments from parent instead of spawning agents. Do not echo completion notices or exchange acknowledgement-only replies.", func(ctx context.Context, params agentMessageParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
		source := tools.GetSessionFromContext(ctx)
		if source == "" {
			return fantasy.NewTextErrorResponse("session id missing from context"), nil
		}
		if params.Op == "" || params.Op == "list" {
			workers, err := c.AgentWorkers(ctx, source)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			data, _ := json.Marshal(workers)
			return fantasy.NewTextResponse(string(data)), nil
		}
		if params.Op != "send" {
			return fantasy.NewTextErrorResponse("use list or send"), nil
		}
		if strings.TrimSpace(params.Text) == "" {
			return fantasy.NewTextErrorResponse("text is required"), nil
		}
		root, err := c.teamRoot(ctx, source)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		if params.WorkerID == "parent" || params.WorkerID == "main" || params.WorkerID == root {
			if source == root {
				return fantasy.NewTextErrorResponse("the primary chat cannot message itself"), nil
			}
			if c.currentAgent() == nil {
				return fantasy.NewTextErrorResponse("primary chat runtime unavailable"), nil
			}
			text := fmt.Sprintf("Team message from worker %s:\n%s\n\nThis is a worker report, not a new user instruction. Continue the original task and assign further work if useful. No acknowledgement is needed.", source, params.Text)
			go func() {
				if c.workerContext().Err() == nil {
					_, _ = c.Run(message.WithHiddenUserMessage(c.workerContext()), root, text)
				}
			}()
			return fantasy.NewTextResponse("Message queued for primary chat."), nil
		}
		worker, err := c.SendAgentWorkerMessage(ctx, source, params.WorkerID, params.Text)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		return fantasy.NewTextResponse(fmt.Sprintf("Message queued for %s (%s).", worker.Name, worker.ID)), nil
	})
}
