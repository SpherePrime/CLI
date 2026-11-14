package agent

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const reviewerAssignment = "Each reviewer assignment names the part, its author, the original user request, the acceptance criteria as a numbered list, how to build it, where its UI runs (a URL or the command that starts it; for an app whose UI is a web frontend, the dev server URL with its backend running in the background, so it can be checked in the browser without launching the native app), and the user language. The reviewer applies its review depth by itself; do not ask it for more or less."

var strictReviewPipeline = fmt.Sprintf(`

<strict_review_pipeline>
Maximum reasoning is selected, so this task runs through the strict review pipeline. It replaces the default delegation flow. The exceptions still apply: work yourself only when the user explicitly said not to use agents or the task is trivial.
1. Explore: for an existing codebase whose context is not known yet, launch explore agents and wait for their reports. If the project directory is empty or this is a new project, skip explore and go straight to Build.
2. Split: vertical slices only. One code agent owns a whole feature, page or module with its markup, styles and logic; never split one feature by layer (HTML, CSS, JS). Only truly independent, substantial parts get their own agent; a single page or small site is one part. Shared conventions go into every assignment.
3. Build and review in one launch: in a single message, for every part launch one review agent (agent=review, background=true, unique name) and one code agent (background=true, unique name, reviewer=<that review agent's name>). Reviewers always start as waiting and make no calls until their author hands work over; the assignment you write is delivered to the reviewer together with the author's report. Do not message a reviewer to start it. %s Code agents write code only; they do not test, build-check or screenshot their own work.
4. Fix loop, routed automatically: when an author finishes, its report goes to its reviewer; CHANGES_REQUIRED goes back to the author; the fixed part goes back to the reviewer; this repeats until APPROVED. Do not relay defects, do not message reviewers to start, do not restart the loop. You are woken only for APPROVED verdicts, failures, escalated disputes and parts past the round limit. After %d rounds a part without APPROVED stops if no critical defect remains; report its remaining defects to the user. Past the limit only critical defects go back to the author and the loop continues until they are fixed; if the same critical defect comes back unchanged three rounds in a row (marked "repeat 3"), give the part to a new code agent with the full defect history and link it to the same reviewer.
5. Finish: an APPROVED part is verified; do not review, test or screenshot it again. When every part is APPROVED or stopped, check only changes you made yourself after the reviews (for example integration edits) with one direct check, then report in the main chat what was built, the review rounds and final scores per part, and any part that was stopped.
A verdict counts as APPROVED only when it says VERDICT: APPROVED, lists no critical or major defects and every score is at least 9/10. Treat anything else as CHANGES_REQUIRED. If an author and a reviewer settle a defect by dropping it without evidence, send it back to the reviewer.
</strict_review_pipeline>
`, reviewerAssignment, reviewDepthStrict.rounds())

func reviewedPolicy(depth reviewDepth) string {
	return fmt.Sprintf(`

<review_policy>
Review depth for this task: %s (it follows the selected reasoning level). For every part you delegate, launch its review agent in the same message as its code agent (agent=review, background=true, unique name; the code agent gets reviewer=<that name>). %s The fix loop is routed automatically and a part gets at most %d review rounds; past the limit only critical defects go back to the author. An APPROVED part is verified: do not check it again; check only your own integration edits once.
</review_policy>
`, depth, reviewerAssignment, depth.rounds())
}

const lightReviewPolicy = `

<review_policy>
Review depth for this task: light (a low reasoning level is selected). Do not launch review agents unless the user asks for a review. When the code agents finish, check the integrated result yourself in one pass: build and the relevant tests once, the main flow through its real surface, and for visible work one browser_run (in Desktop) or Playwright / Chrome / browser MCP check (in CLI) or app check with a desktop and a phone screenshot plus a close-up of anything that looks off. Send only critical problems back to the author with agent_message; do not run a second full pass.
</review_policy>
`

func desktopHost() bool {
	return os.Getenv("PRIME_DESKTOP") == "1"
}

func reviewPolicyBlock(coordinatorRun bool, mode string) string {
	if !coordinatorRun {
		return ""
	}
	switch depth := reviewDepthFor(mode); depth {
	case reviewDepthStrict:
		return strictReviewPipeline
	case reviewDepthLight:
		return lightReviewPolicy
	default:
		return reviewedPolicy(depth)
	}
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
