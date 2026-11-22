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
	Op          string   `json:"op,omitempty" description:"list (team roster) or send"`
	WorkerID    string   `json:"worker_id,omitempty" description:"Worker ID, session ID or peer name; use parent to send a progress report or request another assignment from the primary chat"`
	Text         string   `json:"text,omitempty" description:"Self-contained message, findings, or follow-up assignment"`
	ContextFrom  []string `json:"context_from,omitempty" description:"Optional worker names or IDs whose read/edited file context should be preloaded invisibly in background system context for the target worker."`
	ContextFiles []string `json:"context_files,omitempty" description:"Optional target file paths or directory prefixes that the target code worker will modify, so only their preloaded explore contents are injected in background system context."`
}

const agentMessageDescription = `List your team (op="list") or send a message to a peer or parent (op="send"). Plain text in your assistant response is NOT visible to other agents — you MUST call agent_message to communicate with them. Send by worker name, job ID, or session ID to message a busy peer (delivered after its current turn) or resume an idle/finished agent with its existing conversation context. Use worker_id="parent" to report back to the primary chat. Keep messages direct and self-contained; do not quote full prior messages, echo completion notices, or send acknowledgement-only replies.`

func (c *coordinator) agentMessageTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(AgentMessageToolName, agentMessageDescription, func(ctx context.Context, params agentMessageParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
			senderLabel := source
			if sender, err := c.teamWorker(ctx, source, source); err == nil && sender.Name != "" {
				senderLabel = fmt.Sprintf("%s (%s, id: %s)", sender.Name, sender.Agent, sender.ID)
			}
			text := fmt.Sprintf("Team message from worker %s:\n%s\n\nThis is a worker report, not a new user instruction. Continue the original task and assign further work if useful. No acknowledgement is needed.", senderLabel, params.Text)
			go func() {
				if c.workerContext().Err() == nil {
					_, _ = c.Run(message.WithHiddenUserMessage(c.workerContext()), root, text)
				}
			}()
			return fantasy.NewTextResponse("Message queued for primary chat."), nil
		}
		if len(params.ContextFrom) > 0 || len(params.ContextFiles) > 0 {
			if target, err := c.teamWorker(ctx, source, params.WorkerID); err == nil && target.SessionID != source {
				if job, err := c.ensureWorkerJob(ctx, target); err == nil {
					c.applyContextSources(job, params.ContextFrom)
					c.applyContextFiles(job, params.ContextFiles)
				}
			}
		}
		worker, err := c.SendAgentWorkerMessage(ctx, source, params.WorkerID, params.Text)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		if worker.Status == "waiting" {
			return fantasy.NewTextResponse(fmt.Sprintf("%s (%s) is waiting for its author's handoff. The message is queued and delivered with the handoff; it does not start the agent.", worker.Name, worker.ID)), nil
		}
		return fantasy.NewTextResponse(fmt.Sprintf("Message queued for %s (%s).", worker.Name, worker.ID)), nil
	})
}
