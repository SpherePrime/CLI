package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
)

type strictVerifyContextKey struct{}

func WithStrictVerify(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, strictVerifyContextKey{}, enabled)
}

func StrictVerifyFromContext(ctx context.Context) (bool, bool) {
	if ctx == nil {
		return false, false
	}
	enabled, ok := ctx.Value(strictVerifyContextKey{}).(bool)
	return enabled, ok
}

func isEnhancedVerificationReasoning(mode string) bool {
	switch config.NormalizeReasoningMode(mode) {
	case config.ReasoningModeXHigh, config.ReasoningModeMax:
		return true
	default:
		return false
	}
}

func strictVerifyPromptBlockWithDelegation(agentID string, enabled bool, allowDelegation bool) string {
	if !enabled {
		return ""
	}
	return strictVerifyPromptBlock(agentID, allowDelegation)
}

func verificationProtocolBlock(agentID string, strictVerify bool, reasoningMode string, allowDelegation bool) string {
	if strictVerify {
		return strictVerifyPromptBlockWithDelegation(agentID, true, allowDelegation)
	}
	if !isEnhancedVerificationReasoning(reasoningMode) {
		return ""
	}
	return enhancedVerificationPromptBlock(agentID)
}

func enhancedVerificationPromptBlock(agentID string) string {
	switch agentID {
	case config.AgentGeneral, config.AgentCode:
		return `

<verification_protocol>
Enhanced verification is active for high reasoning levels:
- Respect narrow operational requests: if the user only asked to build and return a path, run a command, or answer a question without changing code, execute it and return immediately (fix only if the requested command fails).
- Fresh evidence after the final edit: rerun affected builds, lint, tests, and real-surface checks after your final edit (desktop widths for PC-only apps — do not test mobile phone widths unless the app targets phones; interactive browser/app checks for UI logic-only changes without mandatory screenshots; unzoomed overview first when visuals change).
- Zero half-finished work: no TODO/FIXME stubs, silenced errors, or fake fallbacks; inspect your full diff for regressions and edge cases and fix root causes before finishing.
</verification_protocol>`
	default:
		return ""
	}
}

func strictVerifyPromptBlock(agentID string, allowDelegation bool) string {
	switch agentID {
	case config.AgentGeneral:
		if allowDelegation {
			return `

<strict_verification>
Strict Verify mode is ENABLED (mandatory quality gate).
1. Skip for questions, operational commands, and trivial edits:
   - Pure questions, read-only research, or narrow operational requests with 0 code edits (such as "just build and give me the path") require NO extra verification and NO review agent — execute and return immediately unless the command fails.
   - Trivial single-file edits (1 file, <= 10 changed lines, no new files) only require a fresh local verification pass.
2. Mandatory independent review for all non-trivial changes:
   - Hand the finished work to an independent review subagent (` + "`agent=\"review\"`" + `) with fresh context (REVIEW DEPTH: strict), giving the exact user requirements, changed files, and target surface.
   - Do NOT accept non-trivial work without ` + "`VERDICT: APPROVED`" + ` obtained AFTER your final edit (0 critical/major defects, scores >= 9/10).
   - On ` + "`CHANGES_REQUIRED`" + `, fix root causes (no TODOs/stubs/silenced errors) and re-submit until ` + "`VERDICT: APPROVED`" + ` (stop early only on a genuine blocker or repeated stall and report remaining defects honestly).
</strict_verification>`
		}
		return `

<strict_verification>
Strict Verify mode is ENABLED (direct mode without delegation).
- Skip for pure questions and narrow operational requests with 0 code edits (execute and return immediately unless the command fails).
- For any code or artifact change, verify strictly with fresh evidence AFTER your final edit: zero TODO/FIXME stubs or silenced errors, green build/lint/tests, and real-surface execution (appropriate widths for the product, interactive clicks for UI logic, unzoomed overview screenshots for visual edits). Fix every defect and repeat until a clean pass completes.
</strict_verification>`
	case config.AgentCode:
		return `

<strict_verification>
Strict Verify mode is ENABLED:
- Deliver production-ready code with zero TODO/FIXME stubs, placeholders, mock fallbacks, or silenced errors.
- Run fresh verification AFTER your final edit matched to the target surface and review your full diff for edge cases and regressions before handing off for strict independent review.
</strict_verification>`
	case config.AgentReview:
		return `

<strict_verification>
Strict Verify mode is ENABLED (` + "`REVIEW DEPTH: strict`" + `):
- Approve ONLY with zero critical defects, zero major defects, no noticeable minor defects, no leftover TODO/FIXME stubs or silenced errors, and every score >= 9/10 backed by fresh evidence collected AFTER the author's last edit.
- Match checks to the target surface: skip mobile phone widths (320/375 px) for desktop/PC-only apps; verify real clicks/inputs/console for UI logic-only changes without redundant screenshots; for visual changes inspect unzoomed overview screenshots first and use zoom ONLY when a small detail or suspected defect genuinely needs closer inspection.
</strict_verification>`
	case config.AgentPlan:
		return `

<strict_verification>
Strict Verify mode is ENABLED:
Every implementation plan for non-trivial changes MUST include target-surface-appropriate verification steps AND a mandatory independent review stage (` + "`agent=\"review\"`" + `) looping until ` + "`VERDICT: APPROVED`" + ` after the final edit.
</strict_verification>`
	default:
		return ""
	}
}
