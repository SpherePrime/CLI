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
	require.Contains(t, explore, "Check if the directory is empty first")
	require.Contains(t, explore, "greenfield project")

	review := renderRolePrompt(t, reviewPrompt)
	require.Contains(t, review, "independent quality gate")
	require.Contains(t, review, "REVIEW DEPTH")
	require.Contains(t, review, "A re-review is not")
	require.Contains(t, review, "Never fix anything yourself")
	require.Contains(t, review, "VERDICT: APPROVED or CHANGES_REQUIRED")
	require.Contains(t, review, "LIKED:")
	require.Contains(t, review, "screenshots")
	require.Contains(t, review, "ROUND:")
	require.Contains(t, review, "Never start reviewing on your own")
	require.Contains(t, review, "routed automatically")
	require.Contains(t, review, "never poll files")
	require.Contains(t, review, "Never wait with sleep")
	require.Contains(t, review, "(repeat N)")
	require.Contains(t, review, "CRITERIA:")
	require.Contains(t, review, "Use it, do not only look at it")
	require.Contains(t, review, "Screenshots go from overview to close-ups")
	require.Contains(t, review, "check the UI in the built-in browser")
	require.Contains(t, review, "- light: one fast pass")
	require.Contains(t, review, "- thorough:")
	require.Contains(t, explore, "Never split one feature by layer")

	code := renderRolePrompt(t, codePrompt)
	require.Contains(t, code, "checking it is not your job")
	require.Contains(t, code, "Do not run tests, builds, linters")
	require.Contains(t, code, "STATUS: WAITING")
	require.Contains(t, code, "do not message the reviewer yourself")
	require.Contains(t, code, "agree shared interfaces")
	require.Contains(t, code, "acceptance criteria one by one")
	require.NotContains(t, code, "Run the project's relevant tests")
}

func TestCoderPromptDelegatesByDefault(t *testing.T) {
	got := renderCoderPrompt(t, false)
	require.Contains(t, got, "delegation is the default")
	require.Contains(t, got, "explicitly says not to use agents")
	require.Contains(t, got, "agent=explore")
	require.Contains(t, got, "empty or new project")
	require.Contains(t, got, "vertical slices, never by layer")
	require.Contains(t, got, "Agents talk to each other directly with agent_message")
	require.NotContains(t, got, "Handle small tasks yourself")
}
