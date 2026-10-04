package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// renderCoderPrompt builds the coder system prompt against a hermetic config,
// with the smart-tools capability mode on or off, since the template branches
// on it and a section dropped on one branch but not the other would go unseen.
func renderCoderPrompt(t *testing.T, smartTools bool) string {
	t.Helper()

	env := testEnv(t)
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
	cfg.SetupAgents()

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)

	out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
	require.NoError(t, err)
	return out
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
				"<when_stuck>",
				"<mcp_usage>",
				"<code_comprehension>",
			} {
				require.Contains(t, got, section,
					"%s is missing with smartTools=%v", section, smartTools)
			}

			// Delegation has to be the default move, not a fallback the model
			// reaches for only when a search fails.
			require.Contains(t, got, "Delegate aggressively")
			require.Contains(t, got, "general")

			// Anti-looping has to state the concrete rule, not just be careful.
			require.Contains(t, got, "Never send the same failing call twice")

			// Identity: a coding agent, stated before anything else.
			require.Contains(t, got, "AI coding agent")
		})
	}
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
