package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

type AgentParams struct {
	Prompt string `json:"prompt" description:"What the agent has to accomplish, stated as a complete task with its own context and a concrete deliverable. It cannot see this conversation, so anything it needs must be in here."`
	Agent  string `json:"agent,omitempty" description:"Which configured agent to run. Omit to use the default agent."`
	// Background starts the agent without waiting for it: the call returns
	// immediately with a job id and the agent keeps running while the caller
	// does other work. Poll it with the agent_jobs tool. Use it to launch
	// many agents at once instead of waiting on each in turn.
	Background bool `json:"background,omitempty" description:"Return immediately with a job id and let the agent keep running; check it later with agent_jobs. Omit to wait for the result now."`
}

const (
	AgentToolName = "agent"
)

// agentTool returns the sub-agent launcher. The agent it builds is resolved per
// call from the requested name, so the model can send work to a different
// configured agent — and therefore a different model — instead of every
// sub-agent landing on the default one.
func (c *coordinator) agentTool(_ context.Context) (fantasy.AgentTool, error) {
	if _, ok := c.cfg.Config().Agents["task"]; !ok {
		// The default sub-agent is still resolved lazily; only fail here when
		// there is nothing to fall back to at all.
		if len(c.SubAgentNames()) == 0 {
			return nil, errors.New("no sub-agent configured")
		}
	}

	return fantasy.NewParallelAgentTool(
		AgentToolName,
		c.agentToolDescription(),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			agent, err := c.SubAgent(params.Agent)
			if err != nil {
				// Reported as a tool error rather than a transport error so the
				// model can see the valid names and retry.
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("%v. Available agents: %s",
						err, strings.Join(c.SubAgentNames(), ", "))), nil
			}

			// Name the agent in the session title. The child session shows up
			// in the session list, and a run of them all called "New Agent
			// Session" gave no way to tell which was which afterwards.
			name := params.Agent
			if name == "" {
				name = config.AgentTask
			}
			title := "Agent Session: " + name

			sp := subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   title,
			}

			// Background mode returns as soon as the sub-session exists; the
			// job keeps running and is read back through agent_jobs. This is
			// how a message can fan out dozens of agents at once: each call
			// claims only the session-creation instant, not the whole run.
			if params.Background {
				job, err := c.startAgentJob(ctx, sp, name)
				if err != nil {
					return fantasy.NewTextErrorResponse(
						fmt.Sprintf("could not start background agent: %s", err)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf(
					"Background agent started: job %s (agent %s). Keep working; check it later with agent_jobs(op=get, job_id=%s) or agent_jobs(op=list).",
					job.id, name, job.id)), nil
			}

			return c.runSubAgent(ctx, sp)
		},
	), nil
}

// agentToolDescription is the static description plus the roster of agents the
// model may target, each with the model it runs on. Without the roster the
// model cannot choose, because only the names are guessable otherwise.
func (c *coordinator) agentToolDescription() string {
	names := c.SubAgentNames()
	if len(names) == 0 {
		return agentToolDescription
	}

	lines := make([]string, 0, len(names)+1)
	for _, name := range names {
		line := "- " + name
		if model := c.SubAgentModel(name); model != "" {
			line += " (model: " + model + ")"
		}
		lines = append(lines, line)
	}

	return agentToolDescription + "\n\nAvailable agents:\n" + strings.Join(lines, "\n")
}
