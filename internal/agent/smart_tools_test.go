package agent

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

const smartToolsTestConfig = `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`

func writeSmartToolsConfig(t *testing.T, workingDir string) *config.ConfigStore {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "prime.json"), []byte(smartToolsTestConfig), 0o644))
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	return cfg
}

func builtToolByName(t *testing.T, built []fantasy.AgentTool, name string) fantasy.AgentTool {
	t.Helper()
	for _, tool := range built {
		if tool.Info().Name == name {
			return tool
		}
	}
	return nil
}

func builtToolSet(t *testing.T, enabled bool) []fantasy.AgentTool {
	t.Helper()

	coord := newGateTestCoordinator(t, true)
	coord.cfg.Config().Options.SmartTools = enabled
	coord.allSkills = []*skills.Skill{
		{
			Name:          "git-workflow",
			Description:   "Use when committing and rebasing branches",
			Instructions:  "Sign every commit and rebase onto main.",
			SkillFilePath: "/skills/git-workflow/SKILL.md",
		},
	}

	agentCfg := coord.cfg.Config().Agents[config.AgentCoder]
	built, err := coord.buildTools(context.Background(), agentCfg, false)
	require.NoError(t, err)
	return built
}

func toolNamesOf(built []fantasy.AgentTool) []string {
	names := make([]string, 0, len(built))
	for _, tool := range built {
		names = append(names, tool.Info().Name)
	}
	slices.Sort(names)
	return names
}

// TestSmartToolsPaletteGatesSearchTools pins the toggle: the default palette is
// untouched, and Smart Tools adds exactly the three search tools on top of it.
func TestSmartToolsPaletteGatesSearchTools(t *testing.T) {
	defaultNames := toolNamesOf(builtToolSet(t, false))
	smartNames := toolNamesOf(builtToolSet(t, true))

	for _, name := range []string{"search_skills", "search_mcp", "search_tools"} {
		require.NotContains(t, defaultNames, name)
		require.Contains(t, smartNames, name)
	}

	// Nothing the default palette offered disappears in smart mode.
	for _, name := range defaultNames {
		require.Contains(t, smartNames, name)
	}
	require.Len(t, smartNames, len(defaultNames)+3)
}

// TestSearchToolResultsTeachUsage keeps the contract the prompt relies on:
// every hit carries an explicit instruction on how to use it.
func TestSearchToolResultsTeachUsage(t *testing.T) {
	built := builtToolSet(t, true)

	searchSkills := builtToolByName(t, built, "search_skills")
	require.NotNil(t, searchSkills)
	result, err := searchSkills.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  searchSkills.Info().Name,
		Input: `{"query":"commit rebase"}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content, "git-workflow")
	require.Contains(t, result.Content, "/skills/git-workflow/SKILL.md")
	require.Contains(t, result.Content, "How to use:")

	// The built-in tool catalog must not advertise the search tools
	// themselves, otherwise a lookup can only suggest another lookup.
	searchTools := builtToolByName(t, built, "search_tools")
	require.NotNil(t, searchTools)
	recursive, err := searchTools.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-2",
		Name:  searchTools.Info().Name,
		Input: `{"query":"keyword"}`,
	})
	require.NoError(t, err)
	require.NotContains(t, recursive.Content, "search_skills")
	require.NotContains(t, recursive.Content, "search_mcp")
	require.NotContains(t, recursive.Content, "search_tools")
}

// TestCoderPromptSwitchesCapabilitySections verifies the system prompt tells
// the model where capabilities come from in each mode.
func TestCoderPromptSwitchesCapabilitySections(t *testing.T) {
	env := testEnv(t)
	cfg := writeSmartToolsConfig(t, env.workingDir)

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)

	cfg.Config().Options.SmartTools = false
	standard, err := p.Build(context.Background(), "mock", "mock-model", cfg)
	require.NoError(t, err)
	require.Contains(t, standard, "<skills_usage>")
	require.Contains(t, standard, "LOAD MATCHING SKILLS")
	require.NotContains(t, standard, "<tool_search>")

	cfg.Config().Options.SmartTools = true
	searching, err := p.Build(context.Background(), "mock", "mock-model", cfg)
	require.NoError(t, err)
	require.Contains(t, searching, "<tool_search>")
	require.Contains(t, searching, "DISCOVER BEFORE ACTING")
	require.Contains(t, searching, "search_skills")
	require.NotContains(t, searching, "<skills_usage>")

	// Discovery mode has to pay for itself in prompt size.
	require.Less(t, len(searching), len(standard))
}
