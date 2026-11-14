package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRoleDeniesDesktopVerificationMCPsForCodeAgent(t *testing.T) {
	t.Parallel()

	codeAgent := config.Agent{ID: config.AgentCode}
	denied := []string{
		config.DesktopBrowserMCPName,
		"apps",
		"windows",
		"prime-desktop",
		"prime-desktop-2",
		"prime-desktop-10",
		"prime-desktop-browser",
		"prime-desktop-browser-2",
		"prime-desktop-apps",
		"prime-desktop-apps-3",
		"prime-desktop-windows",
		"prime-desktop-windows-4",
		"browser-2",
		"apps-2",
		"windows-2",
	}
	for _, name := range denied {
		require.True(t, roleDeniesMCP(codeAgent, name), "expected code agent to deny %q", name)
	}

	allowed := []string{
		"context7",
		"github",
		"prime-desktop-terminal",
		"prime-desktop-git",
		"prime-desktop-ssh",
		"prime-desktop-",
		"prime-desktop-browser-extra",
		"",
	}
	for _, name := range allowed {
		require.False(t, roleDeniesMCP(codeAgent, name), "expected code agent to allow %q", name)
	}

	for _, roleID := range []string{config.AgentGeneral, config.AgentReview, config.AgentExplore, config.AgentTask, config.AgentPlan} {
		other := config.Agent{ID: roleID}
		for _, name := range denied {
			require.False(t, roleDeniesMCP(other, name), "role %q should not be denied %q by roleDeniesMCP", roleID, name)
		}
	}
}

func TestAgentAllowsMCPServerRespectsRoleAndAllowedMCP(t *testing.T) {
	t.Parallel()

	codeDefault := config.Agent{ID: config.AgentCode}
	require.False(t, agentAllowsMCPServer(codeDefault, "browser"))
	require.False(t, agentAllowsMCPServer(codeDefault, "apps"))
	require.False(t, agentAllowsMCPServer(codeDefault, "windows"))
	require.False(t, agentAllowsMCPServer(codeDefault, "prime-desktop-browser-2"))
	require.True(t, agentAllowsMCPServer(codeDefault, "context7"))

	codeExplicit := config.Agent{
		ID: config.AgentCode,
		AllowedMCP: map[string][]string{
			"browser":  nil,
			"context7": nil,
		},
	}
	require.False(t, agentAllowsMCPServer(codeExplicit, "browser"))
	require.True(t, agentAllowsMCPServer(codeExplicit, "context7"))
	require.False(t, agentAllowsMCPServer(codeExplicit, "other"))

	explore := config.Agent{ID: config.AgentExplore, AllowedMCP: map[string][]string{}}
	require.False(t, agentAllowsMCPServer(explore, "context7"))
	require.False(t, agentAllowsMCPServer(explore, "browser"))

	review := config.Agent{ID: config.AgentReview}
	require.True(t, agentAllowsMCPServer(review, "browser"))
	require.True(t, agentAllowsMCPServer(review, "apps"))
	require.True(t, agentAllowsMCPServer(review, "windows"))
	require.True(t, agentAllowsMCPServer(review, "prime-desktop-browser"))
	require.True(t, agentAllowsMCPServer(review, "context7"))
}

func TestAgentToolAppendsRuntimeRestrictionForCodeAgent(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.SetupAgents()
	c.permissions = env.permissions

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	var seenPrompts []string
	worker := newMockAgent("job-provider", 1024, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		seenPrompts = append(seenPrompts, call.Prompt)
		return agentResultWithText("done"), nil
	})
	c.subAgents = map[string]SessionAgent{
		config.AgentCode: worker,
		config.AgentTask: worker,
	}

	tool, err := c.agentTool(t.Context())
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "msg-1")

	callTool := func(id string, params AgentParams) {
		input, marshalErr := json.Marshal(params)
		require.NoError(t, marshalErr)
		resp, runErr := tool.Run(ctx, fantasy.ToolCall{ID: id, Name: AgentToolName, Input: string(input)})
		require.NoError(t, runErr)
		require.False(t, resp.IsError, resp.Content)
	}

	callTool("c1", AgentParams{
		Name:     "ui",
		Prompt:   "Implement the form. Verification criteria: build passes.",
		Agent:    config.AgentCode,
		Reviewer: "ui-review",
	})
	require.Len(t, seenPrompts, 1)
	require.Contains(t, seenPrompts[0], "Implement the form.")
	require.Contains(t, seenPrompts[0], "Runtime restriction: you are a code agent")
	require.Contains(t, seenPrompts[0], "Do NOT run tests, builds, typechecks, linters, dev servers, browsers, app checks or audit scripts")
	require.Contains(t, seenPrompts[0], "Review agent ui-review will build, run and verify your work.")

	callTool("c2", AgentParams{
		Name:   "api",
		Prompt: "Implement the API endpoint.",
	})
	require.Len(t, seenPrompts, 2)
	require.Contains(t, seenPrompts[1], "Runtime restriction: you are a code agent")
	require.Contains(t, seenPrompts[1], "The coordinator or a review agent will build, run and verify your work.")

	callTool("c3", AgentParams{
		Name:   "lookup",
		Prompt: "Find where routes are defined.",
		Agent:  config.AgentTask,
	})
	require.Len(t, seenPrompts, 3)
	require.NotContains(t, seenPrompts[2], "Runtime restriction: you are a code agent")
}
