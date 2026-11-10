package agent

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const strictReviewMaxRounds = 10

var strictReviewPipeline = fmt.Sprintf(`

<strict_review_pipeline>
Maximum reasoning is selected in Prime Desktop, so this task runs through the strict review pipeline. It replaces the default delegation flow. The exceptions still apply: work yourself only when the user explicitly said not to use agents or the task is trivial.
1. Explore: for an existing codebase whose context is not known yet, launch explore agents and wait for their reports. If the project directory is empty or this is a new project, skip explore and go straight to Build.
2. Split: vertical slices only. One code agent owns a whole feature, page or module with its markup, styles and logic; never split one feature by layer (HTML, CSS, JS). Only truly independent, substantial parts get their own agent; a single page or small site is one part. Shared conventions go into every assignment.
3. Build and review in one launch: in a single message, for every part launch one review agent (agent=review, background=true, unique name) and one code agent (background=true, unique name, reviewer=<that review agent's name>). Reviewers launched while code agents run are registered as waiting and make no calls until their author hands work over. Each reviewer assignment names the part, its author, the original user request, the acceptance criteria, how to build and run it, and the user language, and tells it to check as strictly as possible, take and inspect screenshots of every screen, state and width, judge for itself whether it likes the result, and answer in its verdict format. Code agents write code only; they do not test, build-check or screenshot their own work.
4. Fix loop, routed automatically: when an author finishes, its report goes to its reviewer; CHANGES_REQUIRED goes back to the author; the fixed part goes back to the reviewer; this repeats until APPROVED. Do not relay defects, do not message reviewers to start, do not restart the loop. You are woken only for APPROVED verdicts, failures, escalated disputes and parts past the round limit. After %d rounds a part without APPROVED stops if no critical defect remains; report its remaining defects to the user. A part with a critical defect keeps looping past the limit; if the same critical defect comes back unchanged three rounds in a row (marked "repeat 3"), give the part to a new code agent with the full defect history and link it to the same reviewer.
5. Finish: when every part is APPROVED or stopped, run your own final check of the integrated whole, then report in the main chat what was built, the review rounds and final scores per part, and any part that was stopped.
A verdict counts as APPROVED only when it says VERDICT: APPROVED, lists no critical or major defects and every score is at least 9/10. Treat anything else as CHANGES_REQUIRED. If an author and a reviewer settle a defect by dropping it without evidence, send it back to the reviewer.
</strict_review_pipeline>
`, strictReviewMaxRounds)

func desktopHost() bool {
	return os.Getenv("PRIME_DESKTOP") == "1"
}

func strictReviewMode(reasoningMode string) bool {
	return desktopHost() && reasoningMode == config.ReasoningModeMax
}

func strictReviewBlock(coordinatorRun bool, model config.SelectedModel) string {
	if !coordinatorRun || !strictReviewMode(model.EffectiveReasoningMode()) {
		return ""
	}
	return strictReviewPipeline
}

func hasAgentTool(agentTools []fantasy.AgentTool) bool {
	return slices.ContainsFunc(agentTools, func(tool fantasy.AgentTool) bool {
		return tool.Info().Name == AgentToolName
	})
}

func (c *coordinator) activeCodeAgents(rootSessionID string) int {
	active := 0
	for _, job := range c.registrySnapshot() {
		job.mu.Lock()
		if job.rootSessionID == rootSessionID && job.agent == config.AgentCode && (job.status == "running" || job.status == "queued") {
			active++
		}
		job.mu.Unlock()
	}
	return active
}

func (c *coordinator) reviewStartsDormant(ctx context.Context, sessionID string) bool {
	root, err := c.teamRoot(ctx, sessionID)
	if err != nil {
		return false
	}
	return c.activeCodeAgents(root) > 0
}
