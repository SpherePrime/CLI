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
	require.Contains(t, explore, "answer them precisely")

	review := renderRolePrompt(t, reviewPrompt)
	require.Contains(t, review, "strict, independent quality gate")
	require.Contains(t, review, "Never fix anything yourself")
	require.Contains(t, review, "VERDICT: APPROVED or CHANGES_REQUIRED")
	require.Contains(t, review, "LIKED:")
	require.Contains(t, review, "screenshots")
	require.Contains(t, review, "ROUND:")
	require.Contains(t, review, "send the author your complete report")
	require.NotContains(t, review, "Send nothing to the author")

	code := renderRolePrompt(t, codePrompt)
	require.Contains(t, code, "When a reviewer sends you defects, fix every one")
	require.Contains(t, code, "message that same reviewer directly")
	require.Contains(t, code, "agree shared interfaces")
}

func TestCoderPromptDelegatesByDefault(t *testing.T) {
	got := renderCoderPrompt(t, false)
	require.Contains(t, got, "delegation is the default")
	require.Contains(t, got, "explicitly says not to use agents")
	require.Contains(t, got, "agent=explore")
	require.Contains(t, got, "Agents talk to each other directly with agent_message")
	require.NotContains(t, got, "Handle small tasks yourself")
}
