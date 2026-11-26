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
0. Respect narrow operational requests: if the user only asked to build and return a path, run a command, or answer a question without changing code, execute that action and return the result immediately without running extra verification or opening a browser (fix only if the requested build/command fails).
1. Fresh evidence after the final edit: any edit invalidates prior checks. After fixing a bug, rerun the affected build, lint, tests, and runtime checks before finishing.
2. Match the target surface:
   - Desktop apps and PC-only tools: verify at normal desktop window sizes (and resizable/min width if applicable). Do not test mobile phone widths unless the app targets phones.
   - Public websites, landings, and responsive web apps: verify both desktop and phone widths.
   - UI logic-only changes (no layout/styling edits): exercise the changed controls live in the browser/app (real clicks, typing, state transitions, console/network errors); screenshots are optional unless visual output changed.
   - UI visual or layout changes: exercise the controls live AND inspect overview screenshots plus targeted close-ups only where small details, tight spacing, or anomalies need inspection.
   - CLI, backend, libraries, and non-visual scripts: run build, lint, tests, and live execution with valid, edge, and invalid inputs. Do not open a browser unless the code generates visual output (images, charts, PDFs, HTML, slides).
3. Zero half-finished work: no TODOs, placeholders, or mock fallbacks where real behaviour is expected. Review your diff for regressions, dead code, and edge cases, and fix every issue you find before reporting completion.
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
Strict Verify mode is ENABLED. This is a mandatory quality gate.
1. Skip for questions, operational commands, and trivial edits:
   - Pure questions, explanations, read-only research, or narrow operational requests with 0 code edits (such as "just build it and give me the path", running a command, or committing) require NO extra verification and NO review agent — execute the request and return the result immediately (only fix if the requested build/command itself fails).
   - Trivial single-file edits (1 file, <= 10 changed lines, no new files) only require a fresh local verification pass (build/test/diagnostics or live click check if UI logic changed).
2. Mandatory independent review for all non-trivial changes:
   - Before reporting completion, you MUST hand the finished work to an independent review subagent (` + "`agent=\"review\"`" + `) with fresh context (REVIEW DEPTH: strict).
   - Provide the user's exact requirements, the changed files, and the target surface (desktop-only PC app vs responsive web vs CLI/backend).
   - Do NOT accept your own non-trivial work without ` + "`VERDICT: APPROVED`" + ` from the review agent obtained AFTER your final edit.
   - If the review returns ` + "`CHANGES_REQUIRED`" + `, fix every reported defect and send it back for re-review until the review returns ` + "`VERDICT: APPROVED`" + ` with zero critical/major defects (scores >= 9/10).
   - Stop early only if a genuine blocker or repeated stall prevents further progress; then report the exact unresolved defects honestly.
3. Fresh evidence & target-surface discipline:
   - Any edit after approval invalidates that approval and requires a re-review.
   - Match checks to the real product: do NOT waste time on phone widths for desktop/PC-only software; DO test real interactive clicks/inputs/console for UI logic changes even when design was untouched (without unnecessary screenshot spam); take targeted close-up screenshots only when visuals/layout changed and details need inspection.
</strict_verification>`
		}
		return `

<strict_verification>
Strict Verify mode is ENABLED (direct mode without delegation). This is a mandatory quality gate.
- Pure questions and narrow operational requests with 0 code edits (such as "just build and return the path") skip this gate — perform the action and return the result immediately unless the build/command itself fails.
- For any code or artifact change, verify strictly with fresh evidence AFTER your final edit: zero TODOs/stubs/mocks, build/lint/tests green, live execution against the real surface (desktop widths only for PC apps; desktop + phone for responsive web; real clicks/inputs/console for UI logic changes without mandatory screenshots; overview + targeted close-ups when visuals changed; live CLI/API runs for non-visual code).
- Fix every defect you find and repeat until a clean pass completes after the final edit.
</strict_verification>`
	case config.AgentCode:
		return `

<strict_verification>
Strict Verify mode is ENABLED:
- Deliver production-ready code with zero TODOs, stubs, placeholders, or mock data.
- Run fresh verification AFTER your final edit matched to the target surface: build, lint, tests, and real execution (live clicks/inputs/console in browser/app for UI logic changes; add overview + targeted close-ups only when visuals/layout changed; skip mobile phone widths for PC/desktop-only software).
- Read your full diff for edge cases, regressions, and project convention violations before handing off for independent strict review.
</strict_verification>`
	case config.AgentReview:
		return `

<strict_verification>
Strict Verify mode is ENABLED. Enforce the highest quality bar (` + "`REVIEW DEPTH: strict`" + `):
- Approve ONLY with zero critical defects, zero major defects, no noticeable minor defects, and every score >= 9/10.
- Every claim must be backed by fresh evidence collected AFTER the author's last edit.
- Be strict AND smart about the target surface:
  - Do NOT demand mobile phone widths (320/375 px) for desktop/PC-only apps; test realistic desktop/resizable widths instead.
  - When only UI logic changed (no styling/layout changes), verify interactive clicks, inputs, state changes, and console/network errors without demanding redundant close-up screenshots.
  - When visuals/layout changed, inspect normal unzoomed overview screenshots first; do NOT zoom by default — use zoom or a close-up ONLY when a small element, fine text/icon, or suspected defect genuinely needs closer inspection.
  - For CLI/backend/libraries/scripts without UI, verify build, lint, tests, and real execution with valid, edge, and invalid inputs.
</strict_verification>`
	case config.AgentPlan:
		return `

<strict_verification>
Strict Verify mode is ENABLED:
Every implementation plan for non-trivial changes MUST include target-surface-appropriate verification steps (live interactive clicks for UI logic, visual checks only when layout/design changes, desktop widths for PC apps, responsive widths for mobile/web apps, real execution for CLI/backend) AND a mandatory independent review stage (` + "`agent=\"review\"`" + `) looping until ` + "`VERDICT: APPROVED`" + ` after the final edit.
</strict_verification>`
	default:
		return ""
	}
}
