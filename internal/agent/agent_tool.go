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

type backgroundAgentMetadata struct {
	Background bool   `json:"background"`
	JobID      string `json:"job_id"`
}

type AgentParams struct {
	Name   string `json:"name,omitempty" description:"Short descriptive agent name identifying its assignment"`
	Prompt string `json:"prompt" description:"What the agent has to accomplish, stated as a complete task with its own context and a concrete deliverable. It cannot see this conversation, so anything it needs must be in here."`
	Agent  string `json:"agent,omitempty" description:"Which configured agent to run. Omit to use the default agent."`
	Background     bool   `json:"background,omitempty" description:"Return immediately with a job id and let the agent keep running; its result arrives as a team notice. Omit to wait for the result now."`
	WaitForMessage bool   `json:"wait_for_message,omitempty" description:"Register the agent without starting it. It makes no model calls until a message arrives (agent_message or an automatic review handoff), then starts with this assignment plus that message. Implies background."`
	Reviewer       string `json:"reviewer,omitempty" description:"Code agents only: name of the review agent for this part. When this agent finishes, its report is handed to that reviewer automatically, and the reviewer's CHANGES_REQUIRED report comes back to this agent automatically."`
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

			name := params.Agent
			if name == "" {
				name = config.AgentCode
			}
			awaitHandoff := name == config.AgentReview && !params.WaitForMessage && (params.Background || c.reviewStartsDormant(ctx, sessionID))
			dormant := params.WaitForMessage || awaitHandoff
			agent, err := c.SubAgent(name)
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
			title := "Agent Session: " + name
			if workerName := strings.TrimSpace(params.Name); workerName != "" {
				title += " | " + workerName
			}

			sp := subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   title,
				Dormant:        dormant,
				AwaitHandoff:   awaitHandoff,
			}
			if name == config.AgentCode {
				sp.Reviewer = strings.TrimSpace(params.Reviewer)
				sp.Prompt = appendCodeAgentRuntimeRule(sp.Prompt, sp.Reviewer)
			}

			// Background mode returns as soon as the sub-session exists; the
			// job keeps running and is read back through agent_jobs. This is
			// how a message can fan out dozens of agents at once: each call
			// claims only the session-creation instant, not the whole run.
			if params.Background || dormant {
				job, err := c.startAgentJob(ctx, sp, name)
				if err != nil {
					return fantasy.NewTextErrorResponse(
						fmt.Sprintf("could not start background agent: %s", err)), nil
				}
				if dormant {
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
						"Agent registered and waiting: job %s (agent %s). It makes no calls until work is handed to it: the automatic handoff from a code agent launched with reviewer=%q, or, when no code agent is linked to it, the moment every code agent has finished. Messages sent to it before that are queued, not delivered. Do not poll it or message it to start.",
						job.id, name, strings.TrimSpace(params.Name))), backgroundAgentMetadata{Background: true, JobID: job.id}), nil
				}
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
					"Agent started: job %s (agent %s). Do not poll it: its result and messages arrive automatically as a team notice after your turn ends. Continue with other assignments or end your turn.",
					job.id, name)), backgroundAgentMetadata{Background: true, JobID: job.id}), nil
			}

			job, err := c.launchAgentJob(ctx, sp, name, false)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if !job.wait(ctx, 0) {
				_ = c.StopAgentWorker(context.Background(), sessionID, job.id)
				return fantasy.NewTextErrorResponse(ctx.Err().Error()), nil
			}
			job.mu.Lock()
			job.seen = true
			status, output := job.status, job.output
			job.mu.Unlock()
			if status != "done" {
				return fantasy.NewTextErrorResponse(output), nil
			}
			return fantasy.NewTextResponse(output), nil
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

func appendCodeAgentRuntimeRule(prompt, reviewer string) string {
	verifier := "The coordinator or a review agent"
	if reviewer != "" {
		verifier = "Review agent " + reviewer
	}
	return strings.TrimRight(prompt, "\r\n") + "\n\nRuntime restriction: you are a code agent — write code only. Do NOT run tests, builds, typechecks, linters, dev servers, browsers, app checks or audit scripts, even if verification criteria appear in the task text above. " + verifier + " will build, run and verify your work."
}
