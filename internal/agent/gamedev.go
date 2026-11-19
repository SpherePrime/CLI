package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
)

type gameDevContextKey struct{}

func WithGameDev(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, gameDevContextKey{}, enabled)
}

func GameDevFromContext(ctx context.Context) (bool, bool) {
	if ctx == nil {
		return false, false
	}
	v, ok := ctx.Value(gameDevContextKey{}).(bool)
	return v, ok
}

func gameDevPromptBlock(agentID string, enabled bool) string {
	return gameDevPromptBlockWithDelegation(agentID, enabled, true)
}

func gameDevPromptBlockWithDelegation(agentID string, enabled, canDelegate bool) string {
	if !enabled {
		return ""
	}
	switch agentID {
	case config.AgentDesigner, config.AgentExplore, config.AgentTask:
		return ""
	case config.AgentCode:
		return `

<gamedev_mode>
GameDev mode is enabled.
Whenever you build or modify game logic, mechanics, levels, progression, or physics, you MUST write automated playthrough test bots (headless simulation/playthrough bots or automated player agents) that actually play through the game from start to finish and across key game loops to:
1. Detect bugs, crashes, unhandled errors, and softlocks or unreachable states.
2. Evaluate and verify difficulty balance (win/loss rates, pacing, resource curves, enemy/obstacle tuning, and progression fairness).
Continue to obey your code-agent rule: do NOT run tests, builds, or the bots yourself; the review agent or coordinator will execute them.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled.
Verification MUST include running and inspecting automated playthrough test bots for the game:
1. Check that automated playthrough test bots exist for the touched game flows and actually play through the game from start to finish / across key game loops.
2. Execute those test bots and verify there are no bugs, crashes, or softlocks during playthroughs.
3. Inspect the test-bot run results and metrics to verify difficulty and balance tuning (no impossible spikes, trivial exploits, or broken progression).
Report missing playthrough test bots, playthrough bugs/softlocks, or failed difficulty/balance checks as defects.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for game features MUST include automated playthrough test bots (headless simulation or automated player agents) that play through the game from start to finish / across key game loops to detect bugs, crashes, and softlocks and to verify difficulty balance.
</gamedev_mode>`
	case config.AgentGeneral, config.LegacyAgentCoder, "":
		if canDelegate {
			return `

<gamedev_mode>
GameDev mode is enabled.
When delegating game implementation or test work:
- Require code agents to write automated playthrough test bots (headless simulation/playthrough bots or automated player agents) that actually play through the game from start to finish / across key game loops to detect bugs, crashes, and softlocks and to verify difficulty balance.
- Require review agents (or your own <completion_check> when no reviewer is used) to execute those automated test bots and verify bug-free playthroughs and balanced difficulty.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled.
Whenever you build or modify game logic, mechanics, levels, progression, or physics, you MUST write and run automated playthrough test bots (headless simulation/playthrough bots or automated player agents) that actually play through the game from start to finish / across key game loops to detect bugs, crashes, and softlocks and to evaluate and verify difficulty balance.
</gamedev_mode>`
	default:
		return ""
	}
}
