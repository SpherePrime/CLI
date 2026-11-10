package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func renderRolePrompt(t *testing.T, build func(...prompt.Option) (*prompt.Prompt, error)) string {
	t.Helper()
	env := testEnv(t)
	p, err := build(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	cfg := renderCoderConfig(t, env)
	out, err := p.Build(context.Background(), "mock", "mock-model", cfg)
	require.NoError(t, err)
	return out
}

func TestRolePromptsDescribeTheirJob(t *testing.T) {
	explore := renderRolePrompt(t, explorePrompt)
	require.Contains(t, explore, "exploration agent")
	require.Contains(t, explore, "Never create, edit, move or delete files")
	require.Contains(t, explore, "Suggested split")

	review := renderRolePrompt(t, reviewPrompt)
	require.Contains(t, review, "strict, independent quality gate")
	require.Contains(t, review, "Never fix anything yourself")
	require.Contains(t, review, "VERDICT: APPROVED or CHANGES_REQUIRED")
	require.Contains(t, review, "LIKED:")
	require.Contains(t, review, "screenshots")

	code := renderRolePrompt(t, codePrompt)
	require.Contains(t, code, "When the coordinator forwards review defects, fix every one")
}

func TestCoderPromptDelegatesByDefault(t *testing.T) {
	got := renderCoderPrompt(t, false)
	require.Contains(t, got, "delegation is the default")
	require.Contains(t, got, "explicitly says not to use agents")
	require.Contains(t, got, "agent=explore")
	require.NotContains(t, got, "Handle small tasks yourself")
}
