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

// renderCoderPrompt builds the coder system prompt against a hermetic config,
// with the smart-tools capability mode on or off, since the template branches
// on it and a section dropped on one branch but not the other would go unseen.
func renderCoderPrompt(t *testing.T, smartTools bool) string {
	t.Helper()

	env := testEnv(t)
	cfg := renderPromptConfig(t, env, smartTools)

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)

	out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
	require.NoError(t, err)
	return out
}

func renderCoderConfig(t *testing.T, env fakeEnv) *config.ConfigStore {
	t.Helper()
	return renderPromptConfig(t, env, false)
}

func renderPromptConfig(t *testing.T, env fakeEnv, smartTools bool) *config.ConfigStore {
	t.Helper()
	primeJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true,
              "smart_tools": ` + boolLiteral(smartTools) + `},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "prime.json"), []byte(primeJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Options.GlobalContextPaths = nil
	cfg.Config().Options.ContextPaths = nil
	cfg.Config().Options.SkillsPaths = nil
	cfg.SetupAgents()
	return cfg
}

func boolLiteral(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// The coder prompt is the only thing a model gets before it touches the
// repository. These sections are the difference between a model that reads
// before editing and one that guesses, and between one that reports a wall and
// one that retries the same failing call until it runs out of context.
func TestCoderPromptCarriesOperatingGuidance(t *testing.T) {
	t.Parallel()

	for _, smartTools := range []bool{false, true} {
		name := "plain"
		if smartTools {
			name = "smartTools"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := renderCoderPrompt(t, smartTools)

			for _, section := range []string{
				"<task_instructions>",
				"# System",
				"# Doing tasks",
				"# Using your tools",
				"# Communicating with the user",
				"# Reporting outcomes",
				"<env>",
			} {
				require.Contains(t, got, section,
					"%s is missing with smartTools=%v", section, smartTools)
			}

			require.Contains(t, got, "search_instructions")
			require.Contains(t, got, "read_instruction")
			require.Contains(t, got, "Never send the exact same failing call twice")
			require.Contains(t, got, "working with the user toward their goals")
			require.Contains(t, got, "Prime is built by SpherePrime")
			require.Contains(t, got, "User and project instructions take priority")
			require.Contains(t, got, "file_path:line_number")
			require.NotContains(t, got, "<whitespace_and_exact_matching>")
			require.NotContains(t, got, "<strict_review_pipeline>")
			require.NotContains(t, got, "delegation is the default")
		})
	}
}

func TestCoderPromptIdentityModes(t *testing.T) {
	t.Run("cli", func(t *testing.T) {
		t.Setenv("PRIME_DESKTOP", "")
		got := renderCoderPrompt(t, false)
		require.Contains(t, got, "You are Prime, an interactive CLI agent working with the user toward their goals")
	})
	t.Run("desktop", func(t *testing.T) {
		t.Setenv("PRIME_DESKTOP", "1")
		got := renderCoderPrompt(t, false)
		require.Contains(t, got, "You are Prime Desktop, an agent in the Prime Desktop application working with the user toward their goals")
	})
}

// The MCP section is worthless if it sends the model after tools that are not
// there, so the names it mentions have to be real ones.
func TestCoderPromptMcpGuidanceNamesRealTools(t *testing.T) {
	t.Parallel()

	got := renderCoderPrompt(t, false)
	for _, tool := range []string{"list_mcp_resources", "read_mcp_resource"} {
		require.Contains(t, got, tool)
	}
}

func TestCoderPromptListsPrimeDesignerSkill(t *testing.T) {
	t.Parallel()

	got := renderCoderPrompt(t, false)
	require.Contains(t, got, "<name>prime-designer</name>")
	require.Contains(t, got, "prime://skills/prime-designer/SKILL.md")
	require.NotContains(t, got, "Design lead and frontend engineer for distinctive, production-ready interfaces and visuals with a measurable quality gate.\n\n# Prime Designer")
	require.NotContains(t, got, "## Anti-slop rules")
	require.Contains(t, got, "read every matching SKILL.md, not only the first")
	require.Contains(t, got, "read prime://skills/prime-designer/SKILL.md before planning")

	smart := renderCoderPrompt(t, true)
	require.Contains(t, smart, "read prime://skills/prime-designer/SKILL.md before planning")
}

func TestCoderPromptWebSearchGuidance(t *testing.T) {
	t.Parallel()

	for _, withWebSearch := range []bool{false, true} {
		name := "without_web_search"
		if withWebSearch {
			name = "with_web_search"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := testEnv(t)
			cfg := renderPromptConfig(t, env, false)
			if withWebSearch {
				pCfg, ok := cfg.Config().Providers.Get("mock")
				require.True(t, ok)
				pCfg.Models = append(pCfg.Models, catwalk.Model{
					ID:      "mock-search",
					Name:    "Mock Search",
					Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"web_search": true}},
				})
				cfg.Config().Providers.Set("mock", pCfg)
			}

			p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
			require.NoError(t, err)

			out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
			require.NoError(t, err)

			if withWebSearch {
				require.Contains(t, out, "web_search is available")
				require.Contains(t, out, "provider's web search capability")
			} else {
				require.Contains(t, out, "For web research, use agentic_fetch when available; fetch reads a specific URL.")
				require.NotContains(t, out, "web_search is available")
			}
		})
	}

	t.Run("disabled_web_search_tool", func(t *testing.T) {
		t.Parallel()

		env := testEnv(t)
		cfg := renderPromptConfig(t, env, false)
		pCfg, ok := cfg.Config().Providers.Get("mock")
		require.True(t, ok)
		pCfg.Models = append(pCfg.Models, catwalk.Model{
			ID:      "mock-search",
			Name:    "Mock Search",
			Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"web_search": true}},
		})
		cfg.Config().Providers.Set("mock", pCfg)
		cfg.Config().Options.DisabledTools = []string{"web_search"}
		cfg.SetupAgents()

		p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
		require.NoError(t, err)

		out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
		require.NoError(t, err)
		require.Contains(t, out, "For web research, use agentic_fetch when available; fetch reads a specific URL.")
		require.NotContains(t, out, "web_search is available")
	})
}

func TestBuildToolsGatingForWebSearch(t *testing.T) {
	t.Parallel()

	coord := newGateTestCoordinator(t, true)
	require.NoError(t, coord.readyWg.Wait())

	// Without a web search model on any provider, web_search is omitted.
	built, err := coord.buildTools(context.Background(), coord.cfg.Config().Agents[config.AgentGeneral], false)
	require.NoError(t, err)
	require.NotContains(t, toolNamesOf(built), "web_search")

	// Add a web search model to the mock provider: web_search is now exposed.
	pCfg, ok := coord.cfg.Config().Providers.Get("mock")
	require.True(t, ok)
	pCfg.Models = append(pCfg.Models, catwalk.Model{
		ID:      "mock-search",
		Name:    "Mock Search",
		Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"web_search": true}},
	})
	coord.cfg.Config().Providers.Set("mock", pCfg)

	built, err = coord.buildTools(context.Background(), coord.cfg.Config().Agents[config.AgentGeneral], false)
	require.NoError(t, err)
	require.Contains(t, toolNamesOf(built), "web_search")

	// Disabling web_search via DisabledTools hides it even when provider supports it.
	coord.cfg.Config().Options.DisabledTools = []string{"web_search"}
	coord.cfg.SetupAgents()
	built, err = coord.buildTools(context.Background(), coord.cfg.Config().Agents[config.AgentGeneral], false)
	require.NoError(t, err)
	require.NotContains(t, toolNamesOf(built), "web_search")
}
