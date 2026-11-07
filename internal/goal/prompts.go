package goal

import (
	"fmt"
	"strings"
)

const CompleteToolName = "goal_complete"

const NoteHeader = "<chat_goal>"

func ContinuationPrompt(progress Progress) string {
	if progress.Finished() {
		return fmt.Sprintf("Goal mode: all %d stages in todos are completed. Verify the whole goal end to end (tests, build, and app_* tools for GUI work). If it is fully met, call %s with a short summary. If something is missing, add the remaining stages to todos and keep working. Do not ask the user whether to continue.", progress.Total, CompleteToolName)
	}
	return fmt.Sprintf("Goal mode: continue with the next stage (%d of %d done). Next: %s. Finish it, verify it, mark it completed in todos, then move on. Do not ask the user whether to continue.", progress.Completed, progress.Total, progress.Next)
}

func Note(current Goal, progress Progress) string {
	var builder strings.Builder
	builder.WriteString("<chat_goal>\n")
	builder.WriteString("This chat has an active goal set by the user:\n")
	builder.WriteString(strings.TrimSpace(current.Text))
	builder.WriteString("\n\n")
	if progress.Total == 0 {
		builder.WriteString("No plan yet. If the goal is ambiguous, first ask the user the clarifying questions you need with the question tool (all in one batch), then break the goal into stages with the todos tool and start the first stage. If nothing is unclear, plan with todos and start right away.\n")
	} else {
		fmt.Fprintf(&builder, "Plan progress: %d of %d stages completed.", progress.Completed, progress.Total)
		if progress.Next != "" {
			fmt.Fprintf(&builder, " Current stage: %s.", progress.Next)
		}
		builder.WriteString(" Resume from this point; do not redo completed stages.\n")
	}
	builder.WriteString("Rules: work through every stage without stopping; verify each stage (tests, build, app_* tools for GUI work) before marking it completed; never end a turn with \"stage done, continue?\" — move on to the next stage yourself. ")
	fmt.Fprintf(&builder, "When every stage is completed and verified, call %s with a short summary. Stop to ask the user only if you are truly blocked.\n", CompleteToolName)
	builder.WriteString("</chat_goal>")
	return builder.String()
}
