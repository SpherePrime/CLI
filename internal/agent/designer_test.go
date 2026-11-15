package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDesignerAgentConfigToolsetAndMCP(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			DisabledTools: []string{},
		},
	}
	cfg.SetupAgents()

	designer, ok := cfg.Agents[config.AgentDesigner]
	require.True(t, ok)
	require.Equal(t, config.AgentDesigner, designer.ID)
	require.Equal(t, "Designer", designer.Name)
	require.Equal(t, config.SelectedModelTypeLarge, designer.Model)
	require.NotNil(t, designer.AllowedMCP)
	require.Empty(t, designer.AllowedMCP)
	require.ElementsMatch(t, []string{"generate_image", "question", "fetch"}, designer.AllowedTools)

	for _, forbidden := range []string{
		"bash", "sudo", "edit", "multiedit", "write", "glob", "grep", "ls",
		"view", "agent", "agent_jobs", "agent_message", "todos", "goal_complete",
		"lsp_diagnostics", "search_instructions", "read_instruction", "search_tools",
	} {
		require.NotContains(t, designer.AllowedTools, forbidden)
	}

	general, ok := cfg.Agents[config.AgentGeneral]
	require.True(t, ok)
	require.Contains(t, general.AllowedTools, "bash")
	require.Contains(t, general.AllowedTools, "edit")
	require.Contains(t, general.AllowedTools, "agent")
}

func isolateDesignerGlobalConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())
}

func TestDesignerBuildToolsIsolatesNonDesignAndMCPTools(t *testing.T) {
	isolateDesignerGlobalConfig(t)

	env := testEnv(t)
	cfg := renderPromptConfig(t, env, true)
	cfg.Config().MCP = config.MCPs{
		"browser": {Type: config.MCPStdio, Command: "echo"},
	}

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		interactive: true,
	}

	built, err := coord.buildTools(context.Background(), cfg.Config().Agents[config.AgentDesigner], false)
	require.NoError(t, err)

	var names []string
	for _, tool := range built {
		names = append(names, tool.Info().Name)
	}
	require.Contains(t, names, "generate_image")
	require.Contains(t, names, "question")
	require.Contains(t, names, "fetch")
	for _, forbidden := range []string{
		"bash", "edit", "multiedit", "write", "glob", "grep", "ls",
		"view", "agent", "agent_jobs", "agent_message", "todos",
		"search_skills", "search_mcp", "search_tools",
	} {
		require.NotContains(t, names, forbidden)
	}
}

func TestDesignerPromptOmitsCodingGitReviewAndDesktopTesting(t *testing.T) {
	for _, withImageModel := range []bool{false, true} {
		name := "without_image_model"
		if withImageModel {
			name = "with_image_model"
		}
		t.Run(name, func(t *testing.T) {
			isolateDesignerGlobalConfig(t)

			env := testEnv(t)
			cfg := renderPromptConfig(t, env, true)
			require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "AGENTS.md"), []byte("SECRET_PROJECT_INSTRUCTIONS"), 0o644))
			cfg.Config().Options.ContextPaths = []string{"AGENTS.md"}

			if withImageModel {
				pCfg, ok := cfg.Config().Providers.Get("mock")
				require.True(t, ok)
				pCfg.Models = append(pCfg.Models, catwalk.Model{ID: "mock-image-1", Name: "Mock Image"})
				cfg.Config().Providers.Set("mock", pCfg)
			}

			p, err := designerPrompt(prompt.WithWorkingDir(env.workingDir))
			require.NoError(t, err)

			out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
			require.NoError(t, err)

			if withImageModel {
				require.Contains(t, out, "generate_image")
				require.Contains(t, out, "source_images")
				require.NotContains(t, out, "prime://settings/providers")
			} else {
				require.Contains(t, out, "prime://settings/providers")
			}

			for _, forbidden := range []string{
				"SECRET_PROJECT_INSTRUCTIONS",
				"<task_instructions>",
				"<desktop_context>",
				"<cli_context>",
				"<skills_usage>",
				"search_instructions",
				"read_instruction",
				"Git repository:",
				"bash",
				"browser_run",
				"VERDICT: APPROVED",
				"todos",
			} {
				require.NotContains(t, out, forbidden)
			}
		})
	}
}

func TestDesignerPrepareSessionPromptSkipsEmptyTodosReminder(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	designerSA := NewSessionAgent(SessionAgentOptions{
		Agent: config.Agent{ID: config.AgentDesigner},
	}).(*sessionAgent)
	generalSA := NewSessionAgent(SessionAgentOptions{
		Agent: config.Agent{ID: config.AgentGeneral},
	}).(*sessionAgent)

	designerHistory, _ := designerSA.prepareSessionPrompt(nil, false, preparePromptOptions{hasTodos: false})
	generalHistory, _ := generalSA.prepareSessionPrompt(nil, false, preparePromptOptions{hasTodos: false})

	require.Empty(t, designerHistory)
	require.NotEmpty(t, generalHistory)
	_ = env
}

func TestNewCoordinatorDefaultsMainAgentToGeneralAndRegistersDesigner(t *testing.T) {
	isolateDesignerGlobalConfig(t)

	env := testEnv(t)
	cfg := renderPromptConfig(t, env, false)

	coordIface, err := NewCoordinator(context.Background(), CoordinatorOptions{
		Config:      cfg,
		Sessions:    env.sessions,
		Messages:    env.messages,
		Permissions: env.permissions,
		History:     env.history,
		FileTracker: *env.filetracker,
		Interactive: true,
	})
	require.NoError(t, err)

	coord := coordIface.(*coordinator)
	require.NoError(t, coord.readyWg.Wait())

	_, activeName := coord.activeAgent()
	require.Equal(t, config.AgentGeneral, activeName)
	require.Contains(t, coord.agents, config.AgentDesigner)
	require.Contains(t, coord.agents, config.AgentGeneral)
	require.Contains(t, coord.agents, config.AgentPlan)
}
