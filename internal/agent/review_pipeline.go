package agent

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const strictReviewMaxRounds = 5

var strictReviewPipeline = fmt.Sprintf(`

<strict_review_pipeline>
Maximum reasoning is selected in Prime Desktop, so this task runs through the strict review pipeline. It replaces the default delegation flow. The exceptions still apply: work yourself only when the user explicitly said not to use agents or the task is trivial.
1. Explore: when the project context is not known yet, launch explore agents and wait for their reports.
2. Build: launch code agents in parallel, one per independent part with no overlapping files, each with its own distinct assignment. Never launch a review agent while any code agent is running or queued: end your turn and wait until every code agent has finished.
3. Review: after all code agents have finished, launch one fresh review agent (agent=review, background=true) per part. Each assignment names the part, its author (agent name and id), the changed files, the original user request, the acceptance criteria, how to build and run it, and the user language. Tell the reviewer to check as strictly as possible, take and inspect screenshots of every screen, state and width, judge for itself whether it likes the result, and answer in its verdict format.
4. Fix loop: on CHANGES_REQUIRED, forward the complete defect list unchanged to the same code agent that built that part with agent_message. When it reports its fixes, send that same reviewer a re-review request with agent_message, including the author's report. Repeat until the verdict is APPROVED. Count the rounds per part; after %d rounds without APPROVED, stop that part and report its remaining defects to the user.
5. Finish: when every part is APPROVED, run your own final check of the integrated whole, then report in the main chat what was built, the review rounds and final scores per part, and any part that was stopped.
A verdict counts as APPROVED only when it says VERDICT: APPROVED, lists no critical or major defects and every score is at least 9/10. Treat anything else as CHANGES_REQUIRED. Never soften, shorten or skip reviewer defects when forwarding them.
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

func (c *coordinator) reviewLaunchBlocked(ctx context.Context, sessionID string) string {
	root, err := c.teamRoot(ctx, sessionID)
	if err != nil {
		return ""
	}
	if !strictReviewMode(c.effectiveTeamReasoning(ctx, root)) {
		return ""
	}
	active := c.activeCodeAgents(root)
	if active == 0 {
		return ""
	}
	return fmt.Sprintf("Strict review pipeline: %d code agent(s) are still running or queued. Review agents start only after every code agent has finished. End your turn; the code agents' results arrive as team notices, then launch the reviewers.", active)
}
