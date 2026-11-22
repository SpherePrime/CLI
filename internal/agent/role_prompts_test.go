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
	require.Contains(t, explore, "file search specialist")
	require.Contains(t, explore, "CRITICAL: READ-ONLY MODE - NO FILE MODIFICATIONS")
	require.Contains(t, explore, "PowerShell")
	require.Contains(t, explore, "empty or contains only an empty `.git` directory")
	require.Contains(t, explore, "absolute paths")
	require.Contains(t, explore, "glob")
	require.Contains(t, explore, "grep")

	review := renderRolePrompt(t, reviewPrompt)
	require.Contains(t, review, "code and product review agent")
	require.Contains(t, review, "REVIEW DEPTH")
	require.Contains(t, review, "A re-review checks only")
	require.Contains(t, review, "Never fix anything yourself")
	require.Contains(t, review, "VERDICT: APPROVED or CHANGES_REQUIRED")
	require.Contains(t, review, "LIKED:")
	require.Contains(t, review, "screenshots")
	require.Contains(t, review, "ROUND:")
	require.Contains(t, review, "Start only when work is handed to you")
	require.Contains(t, review, "routed automatically")
	require.Contains(t, review, "do not wait with sleep or poll in loops")
	require.Contains(t, review, "(repeat N)")
	require.Contains(t, review, "CRITERIA:")
	require.Contains(t, review, "browser_run")
	require.Contains(t, review, "overview to close-ups")
	require.Contains(t, review, "CONFIRMED")
	require.Contains(t, review, "PLAUSIBLE")
	require.Contains(t, review, "REFUTED")
	require.Contains(t, review, "Gap sweep")
	require.Contains(t, review, "- light: one fast pass")
	require.Contains(t, review, "- thorough:")

	code := renderRolePrompt(t, codePrompt)
	require.Contains(t, code, "autonomous implementation worker")
	require.Contains(t, code, "Build, lint, and run the relevant tests")
	require.Contains(t, code, "unexpected local modifications")
	require.Contains(t, code, "file_path:line_number")
	require.Contains(t, code, "agent_message")
	require.Contains(t, code, "read_instruction")
	require.NotContains(t, code, "checking it is not your job")
	require.NotContains(t, code, "Do not run tests, builds, linters")
	require.NotContains(t, code, "STATUS: WAITING")

	task := renderRolePrompt(t, taskPrompt)
	require.Contains(t, task, "general-purpose read-only research")
	require.Contains(t, task, "READ-ONLY")
	require.Contains(t, task, "file_path:line_number")
}

func TestCoderPromptUsesSubagentRestraint(t *testing.T) {
	got := renderCoderPrompt(t, false)
	require.Contains(t, got, "Using subagents")
	require.Contains(t, got, "Do NOT launch subagents excessively")
	require.Contains(t, got, "Avoid duplicating searches")
	require.NotContains(t, got, "delegation is the default")
	require.NotContains(t, got, "never ask a code agent to run tests, builds, linters, dev servers or browsers")
}

func TestAgentToolAndSummaryPromptsAlignWithAutonomousWorkers(t *testing.T) {
	require.Contains(t, agentToolDescription, "# When to launch subagents")
	require.Contains(t, agentToolDescription, "# When NOT to launch subagents")
	require.Contains(t, agentToolDescription, "context_from")
	require.NotContains(t, agentToolDescription, "Send verification requirements only to review agents")
	require.NotContains(t, agentToolDescription, "never ask a code agent to run tests, builds, linters, dev servers or browsers")

	summary := string(summaryPrompt)
	require.Contains(t, summary, "Respond with TEXT ONLY. Do NOT call any tools")
	require.Contains(t, summary, "<analysis>")
	require.Contains(t, summary, "<summary>")
	require.Contains(t, summary, "All User Messages")
	require.Contains(t, summary, "Pending Tasks")
	require.Contains(t, summary, "VERDICT: APPROVED")
	require.NotContains(t, summary, "code agents write code only and must NOT run tests, builds, linters, dev servers, browsers, or audit scripts")
}
