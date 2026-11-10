package agent

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/SpherePrime/CLI/internal/config"
)

var (
	approvedVerdict = regexp.MustCompile(`(?im)^\W*VERDICT:\W*APPROVED\b`)
	changesVerdict  = regexp.MustCompile(`(?i)CHANGES_REQUIRED`)
	criticalDefect  = regexp.MustCompile(`(?i)\[\s*critical\b`)
	waitingReport   = regexp.MustCompile(`(?im)^\W*STATUS:\W*WAITING\b`)
)

type reviewRouteAction struct {
	target  string
	text    string
	note    string
	quiet   bool
	prepare func(*agentJob)
}

func jobActive(status string) bool {
	return status == "running" || status == "queued" || status == "waiting"
}

func (j *agentJob) stop() {
	j.mu.Lock()
	j.pending = nil
	cancel := j.cancel
	waiting := j.status == "waiting"
	j.mu.Unlock()
	if waiting {
		j.finish("canceled", "canceled while waiting for a message")
		return
	}
	if cancel != nil {
		cancel()
	}
}

func reviewApproved(report string) bool {
	return approvedVerdict.MatchString(report) && !changesVerdict.MatchString(report)
}

func (c *coordinator) reviewRoute(job *agentJob, status, output string) *reviewRouteAction {
	if status != "done" {
		return nil
	}
	switch job.agent {
	case config.AgentCode:
		if job.reviewer == "" || waitingReport.MatchString(output) {
			return nil
		}
		authorID := job.id
		return &reviewRouteAction{
			target: job.reviewer,
			text:   authorHandoffText(job.name, job.id, output),
			quiet:  true,
			prepare: func(reviewer *agentJob) {
				reviewer.author = authorID
				reviewer.reviewRound++
			},
		}
	case config.AgentReview:
		if job.author == "" || reviewApproved(output) {
			return nil
		}
		critical := criticalDefect.MatchString(output)
		round := job.reviewRound
		if round >= strictReviewMaxRounds && !critical {
			return &reviewRouteAction{note: fmt.Sprintf("Review round limit (%d) reached without APPROVED and no critical defects remain. Stop this part and report its remaining defects to the user.\n\n", strictReviewMaxRounds)}
		}
		action := &reviewRouteAction{target: job.author, text: fixRequestText(round, output), quiet: round < strictReviewMaxRounds}
		if !action.quiet {
			action.note = fmt.Sprintf("Round %d is past the limit of %d, but critical defects remain, so the fix loop continues. If the same critical defect keeps coming back unchanged, give the part to a new code agent with the full defect history.\n\n", round, strictReviewMaxRounds)
		}
		return action
	}
	return nil
}

func (c *coordinator) deliverReviewRoute(job *agentJob, route *reviewRouteAction) bool {
	if route.target == "" {
		return true
	}
	if _, err := c.sendWorkerMessage(c.workerContext(), job.sessionID, route.target, route.text, route.prepare); err != nil {
		slog.Warn("Review handoff failed", "job", job.id, "target", route.target, "error", err)
		return false
	}
	return true
}

func authorHandoffText(name, id, report string) string {
	return fmt.Sprintf(`Automatic review handoff: author %s (worker id %s) finished its part and the work is ready for review. Review it now as your assignment describes.
Your final answer is routed automatically: CHANGES_REQUIRED goes back to this author for fixes, APPROVED goes to the coordinator. Do not send the report with agent_message.

Author report:
%s`, name, id, report)
}

func fixRequestText(round int, report string) string {
	return fmt.Sprintf(`Automatic review result, round %d. Fix every defect below.
Do not run tests, builds, linters, browsers or screenshots and do not message the reviewer: when you end your turn, your final report goes back to the same reviewer for a re-review automatically.
End with a report that lists each defect and what changed for it. If you believe a defect is wrong, explain it with evidence in the report instead of skipping it silently.

Review report:
%s`, round, report)
}
