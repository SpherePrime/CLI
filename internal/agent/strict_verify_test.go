package agent

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func assertNoStrictVerifyMentions(t *testing.T, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	require.NotContains(t, lower, "<strict_verification>")
	require.NotContains(t, lower, "strict verify")
	require.NotContains(t, lower, "strict_verify")
}

func TestStrictVerifyDisabledAddsNothingToPrompt(t *testing.T) {
	t.Parallel()

	roles := []string{
		config.AgentGeneral,
		config.AgentCode,
		config.AgentReview,
		config.AgentPlan,
		config.AgentDesigner,
		config.AgentExplore,
		config.AgentTask,
	}
	for _, id := range roles {
		require.Empty(t, strictVerifyPromptBlockWithDelegation(id, false, true))
		require.Empty(t, strictVerifyPromptBlockWithDelegation(id, false, false))
		require.Empty(t, verificationProtocolBlock(id, false, config.ReasoningModeMedium, true))
		require.Empty(t, verificationProtocolBlock(id, false, config.ReasoningModeHigh, true))
		require.Empty(t, verificationProtocolBlock(id, false, config.ReasoningModeLow, true))
		require.Empty(t, verificationProtocolBlock(id, false, config.ReasoningModeDefault, true))
	}

	enhanced := verificationProtocolBlock(config.AgentGeneral, false, config.ReasoningModeMax, true)
	require.Contains(t, enhanced, "<verification_protocol>")
	require.Contains(t, enhanced, "Respect narrow operational requests")
	require.Contains(t, enhanced, "do not test mobile phone widths unless the app targets phones")
	require.Contains(t, enhanced, "UI logic-only changes")
	require.Contains(t, enhanced, "TODO/FIXME stubs")
	assertNoStrictVerifyMentions(t, enhanced)
}

func TestStrictVerifyEnabledPromptByRole(t *testing.T) {
	t.Parallel()

	require.Empty(t, strictVerifyPromptBlockWithDelegation(config.AgentDesigner, true, true))
	require.Empty(t, strictVerifyPromptBlockWithDelegation(config.AgentExplore, true, true))
	require.Empty(t, strictVerifyPromptBlockWithDelegation(config.AgentTask, true, true))

	coordBlock := strictVerifyPromptBlockWithDelegation(config.AgentGeneral, true, true)
	require.Contains(t, coordBlock, "<strict_verification>")
	require.Contains(t, coordBlock, "independent review subagent")
	require.Contains(t, coordBlock, "VERDICT: APPROVED")
	require.Contains(t, coordBlock, "<= 10 changed lines")
	require.Contains(t, coordBlock, "target surface")

	directBlock := strictVerifyPromptBlockWithDelegation(config.AgentGeneral, true, false)
	require.Contains(t, directBlock, "<strict_verification>")
	require.Contains(t, directBlock, "direct mode without delegation")

	codeBlock := strictVerifyPromptBlockWithDelegation(config.AgentCode, true, true)
	require.Contains(t, codeBlock, "<strict_verification>")
	require.Contains(t, codeBlock, "TODO/FIXME stubs")
	require.Contains(t, codeBlock, "silenced errors")

	reviewBlock := strictVerifyPromptBlockWithDelegation(config.AgentReview, true, true)
	require.Contains(t, reviewBlock, "<strict_verification>")
	require.Contains(t, reviewBlock, "every score >= 9/10")
	require.Contains(t, reviewBlock, "UI logic-only changes")
	require.Contains(t, reviewBlock, "TODO/FIXME stubs")

	planBlock := strictVerifyPromptBlockWithDelegation(config.AgentPlan, true, true)
	require.Contains(t, planBlock, "<strict_verification>")
	require.Contains(t, planBlock, "independent review stage")
}

func TestVerifyTrackerSkipsQuestionsAndTrivialEdits(t *testing.T) {
	t.Parallel()

	tracker := newRunVerifyTracker()
	_, need := tracker.nextGatePrompt(true, config.ReasoningModeMax, true, 0)
	require.False(t, need)

	tracker.recordMutation("a.go", 3, false)
	require.True(t, tracker.isTrivialMutation())

	prompt, need := tracker.nextGatePrompt(true, config.ReasoningModeMedium, true, 0)
	require.True(t, need)
	require.Contains(t, prompt, "trivial edit")
	require.NotContains(t, prompt, "VERDICT: APPROVED")

	tracker.recordLocalCheck()
	_, need = tracker.nextGatePrompt(true, config.ReasoningModeMedium, true, 1)
	require.False(t, need)
}

func TestVerifyTrackerStrictRequiresFreshApprovalAfterFinalEdit(t *testing.T) {
	t.Parallel()

	tracker := newRunVerifyTracker()
	tracker.recordMutation("a.go", 20, false)
	require.False(t, tracker.isTrivialMutation())

	tracker.recordLocalCheck()
	prompt, need := tracker.nextGatePrompt(true, config.ReasoningModeHigh, true, 0)
	require.True(t, need)
	require.Contains(t, prompt, "VERDICT: APPROVED")

	tracker.recordReviewResult("VERDICT: APPROVED\nDEFECTS: none")
	_, need = tracker.nextGatePrompt(true, config.ReasoningModeHigh, true, 1)
	require.False(t, need)

	tracker.recordMutation("a.go", 2, false)
	_, need = tracker.nextGatePrompt(true, config.ReasoningModeHigh, true, 2)
	require.True(t, need)
}

func TestVerifyTrackerStallDetectorStopsRepeatedReviewDefects(t *testing.T) {
	t.Parallel()

	tracker := newRunVerifyTracker()
	tracker.recordMutation("app.go", 35, false)

	report := "VERDICT: CHANGES_REQUIRED\nDEFECTS:\n- [major] [CONFIRMED] app.go:10 - broken state\nCHECKS: go test"
	tracker.recordReviewResult(report)
	require.False(t, tracker.reviewStalled())
	tracker.recordReviewResult(report)
	require.False(t, tracker.reviewStalled())
	tracker.recordReviewResult(report)
	require.True(t, tracker.reviewStalled())

	_, need := tracker.nextGatePrompt(true, config.ReasoningModeHigh, true, 3)
	require.False(t, need)
}

func TestVerifyTrackerBaseAndEnhancedLevels(t *testing.T) {
	t.Parallel()

	base := newRunVerifyTracker()
	base.recordMutation("x.go", 15, false)
	_, need := base.nextGatePrompt(false, config.ReasoningModeMedium, true, 0)
	require.True(t, need)
	_, need = base.nextGatePrompt(false, config.ReasoningModeMedium, true, 1)
	require.False(t, need)

	enhanced := newRunVerifyTracker()
	enhanced.recordMutation("x.go", 15, false)
	_, need = enhanced.nextGatePrompt(false, config.ReasoningModeMax, true, 0)
	require.True(t, need)
	enhanced.recordMutation("x.go", 2, false)
	_, need = enhanced.nextGatePrompt(false, config.ReasoningModeMax, true, 1)
	require.True(t, need)
	enhanced.recordLocalCheck()
	_, need = enhanced.nextGatePrompt(false, config.ReasoningModeMax, true, 2)
	require.False(t, need)
}

func TestStrictVerifyForcesStrictReviewDepthAndSixRoundsAndStallStop(t *testing.T) {
	t.Parallel()

	c := &coordinator{}
	c.teamStrictVerify.Store("root", true)
	c.teamReasoning.Store("root", config.ReasoningModeLow)

	require.Equal(t, reviewDepthStrict, c.teamReviewDepth("root"))
	require.Equal(t, strictVerifyMaxAttempts, c.reviewRoundLimit("root"))
	require.Contains(t, c.reviewDepthNote("root"), "REVIEW DEPTH: strict")
	require.Contains(t, c.reviewDepthNote("root"), "at most 6 review rounds")

	job := &agentJob{
		agent:         config.AgentReview,
		author:        "code-1",
		rootSessionID: "root",
		reviewRound:   1,
	}
	report := "VERDICT: CHANGES_REQUIRED\nDEFECTS:\n- [major] [CONFIRMED] ui.tsx:12 - click handler broken"
	r1 := c.reviewRoute(job, "done", report)
	require.NotNil(t, r1)
	require.Equal(t, "code-1", r1.target)

	job.reviewRound = 2
	r2 := c.reviewRoute(job, "done", report)
	require.NotNil(t, r2)
	require.Equal(t, "code-1", r2.target)

	job.reviewRound = 3
	r3 := c.reviewRoute(job, "done", report)
	require.NotNil(t, r3)
	require.Empty(t, r3.target)
	require.Contains(t, r3.note, "Strict Verify stopped")
}
