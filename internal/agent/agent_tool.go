package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"

	"github.com/SpherePrime/CLI/internal/agent/tools"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

type AgentParams struct {
	Prompt string `json:"prompt" description:"What the agent has to accomplish, stated as a complete task with its own context and a concrete deliverable. It cannot see this conversation, so anything it needs must be in here."`
	Agent  string `json:"agent,omitempty" description:"Which configured agent to run. Omit to use the default agent."`
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

			title := "New Agent Session"
			if params.Agent != "" {
				title = "Agent Session: " + params.Agent
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   title,
			})
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