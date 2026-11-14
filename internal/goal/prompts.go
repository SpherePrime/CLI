package goal

import (
	"fmt"
	"strings"
)

const CompleteToolName = "goal_complete"

const NoteHeader = "<chat_goal>"

const qualityBar = "The result must be a complete, production-ready product: every requested feature works end to end, no placeholders, stubs, TODOs or mock data, errors and edge cases are handled, the interface is clean and consistent, and build, tests and linters pass."

const acceptanceSteps = "Final acceptance: check integration and any unreviewed changes across the whole product like a strict QA engineer and senior reviewer. Stages or parts already verified with VERDICT: APPROVED by a review agent (and unmodified after review) do NOT need to be re-tested or re-checked from scratch. For unreviewed changes or integration edits, build and run the real product (websites in the built-in browser with browser_* tools, native desktop apps with app_* tools), walk through every affected user flow including empty, invalid and error states by actually clicking and typing, check layout at different sizes with overview screenshots and close-ups, run relevant tests and linters, and review the diff for bugs, dead code and inconsistencies."

func ContinuationPrompt(progress Progress) string {
	if progress.Finished() {
		return fmt.Sprintf("Goal mode: all %d stages in todos are completed. %s %s If you find any problem or anything below this bar, add stages for the fixes to todos, fix them and repeat the acceptance pass. Call %s only when a full acceptance pass finds nothing left to improve. Do not ask the user whether to continue.", progress.Total, acceptanceSteps, qualityBar, CompleteToolName)
	}
	return fmt.Sprintf("Goal mode: continue with the next stage (%d of %d done). Next: %s. Finish it completely, verify it via a review agent (VERDICT: APPROVED) or directly if unreviewed (tests, build, and run it: browser_* for web, app_* for native apps) without re-checking already APPROVED unmodified parts, fix what the check reveals, mark it completed in todos, then move on. Do not ask the user whether to continue.", progress.Completed, progress.Total, progress.Next)
}

func Note(current Goal, progress Progress) string {
	var builder strings.Builder
	builder.WriteString(NoteHeader + "\n")
	builder.WriteString("This chat has an active goal set by the user:\n")
	builder.WriteString(strings.TrimSpace(current.Text))
	builder.WriteString("\n\n")
	builder.WriteString(qualityBar)
	builder.WriteString("\n\n")
	if progress.Total == 0 {
		builder.WriteString("No plan yet. If the goal leaves important decisions open (scope, platform, stack, design, data, integrations), first ask the user all clarifying questions you need with the question tool in one batch. Then break the goal into concrete stages with the todos tool; the plan must end with two stages: \"Final acceptance of the whole product\" and \"Polish and fix everything found\". Then start the first stage.\n")
	} else {
		fmt.Fprintf(&builder, "Plan progress: %d of %d stages completed.", progress.Completed, progress.Total)
		if progress.Next != "" {
			fmt.Fprintf(&builder, " Current stage: %s.", progress.Next)
		}
		builder.WriteString(" Resume from this point; do not redo completed stages.\n")
	}
	builder.WriteString("Rules: work through every stage without stopping; verify each stage (via a review agent's VERDICT: APPROVED or directly when no reviewer is used, without re-checking already APPROVED unmodified parts) before marking it completed; never end a turn with \"stage done, continue?\" — move on to the next stage yourself. ")
	builder.WriteString(acceptanceSteps)
	fmt.Fprintf(&builder, " Fix everything it finds and repeat until a pass is clean, then call %s with a short summary of what was delivered and how it was verified. Stop to ask the user only if you are truly blocked.\n", CompleteToolName)
	builder.WriteString("</chat_goal>")
	return builder.String()
}
