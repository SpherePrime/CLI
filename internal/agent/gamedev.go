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
First, read prime://skills/prime-gamedev/SKILL.md with view to access the live bot runner and video-to-frames inspection scripts.
Whenever you build or modify game logic, mechanics, rendering, collisions, levels, UI/HUD, or progression, you MUST write and run a live automated playthrough bot against the real running game (not unit-test stubs alone):
1. Full-loop bug hunting: make the bot play a full game loop from start to finish while actively probing corners, walls, floor seams, map boundaries, rapid/conflicting inputs, and restart/respawn paths to trigger bugs and softlocks.
2. Scripted telemetry checks: detect falling through floors/textures, collision clipping, out-of-bounds coordinates, NaN/runtime/console/WebGL errors, missing/404 textures or assets, stuck/softlock states, and FPS drops.
3. Video recording and dense frame inspection: record video of the full bot playthrough in a temp directory outside the repo, slice it into a dense frame sequence and contact sheets (using web_bot_runner.mjs or extract_frames.mjs), and inspect the contact sheets and flagged/key frames with view (when image viewing is supported) to catch visual bugs, texture glitches, z-fighting, camera clipping, and HUD overlaps.
4. Difficulty and balance verification: verify win/loss rates, pacing, resource/damage curves, and progression fairness.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled.
First, read prime://skills/prime-gamedev/SKILL.md with view.
Verification MUST include running and visually inspecting a live bug-hunting playthrough bot against the real game:
1. Execute the live playthrough bot across a full game loop and bug-hunting probes (walls, corners, floor seams, edges, rapid inputs, softlock spots) with video recording enabled.
2. Slice the recorded gameplay video into a dense sequence of frames and contact sheets (via web_bot_runner.mjs or extract_frames.mjs in a temp directory outside the repo) and inspect the contact sheets and key/flagged frames with view (when image viewing is supported) to verify there are no visual bugs, texture fall-throughs, missing/broken textures, z-fighting, camera clipping, or HUD overlaps.
3. Inspect scripted runtime/telemetry results to verify zero falls through floors/textures, collision clipping, out-of-bounds states, softlocks, NaN physics, console/WebGL errors, or broken assets.
4. Inspect playthrough metrics to confirm difficulty and balance tuning (no impossible spikes, trivial exploits, or broken progression).
Report missing live playthrough bots, skipped video/frame visual inspection, visual or texture bugs, softlocks, or failed difficulty/balance checks as defects.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for game features MUST reference prime://skills/prime-gamedev/SKILL.md and include:
1. A live automated playthrough bot that plays a full loop and actively hunts for collision, texture-fall-through, and softlock bugs.
2. Scripted runtime telemetry checks (floor/texture clipping, out-of-bounds, NaN physics, console/WebGL/asset errors, FPS).
3. Recording gameplay video of the bot run, slicing it into dense frames and contact sheets, and visually inspecting the frames with view for visual/rendering defects.
4. Difficulty and balance verification across win/loss rates, pacing, and progression curves.
</gamedev_mode>`
	case config.AgentGeneral, config.LegacyAgentCoder, "":
		if canDelegate {
			return `

<gamedev_mode>
GameDev mode is enabled.
Read prime://skills/prime-gamedev/SKILL.md with view and enforce the full GameDev QA workflow when delegating or verifying game work:
- Require code agents to build and run live bug-hunting playthrough bots against the real game, add scripted collision/texture-fall-through/error telemetry, record gameplay video, slice it into dense frames, inspect frames with view, and verify difficulty balance.
- Require review agents (or your own <completion_check> when no reviewer is used) to run the live bot with video recording, slice the video into dense frames, inspect the contact sheets and flagged/key frames with view for visual and texture bugs, and verify scripted collision/error checks and difficulty balance.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled.
First, read prime://skills/prime-gamedev/SKILL.md with view to access the live bot runner and video-to-frames inspection scripts.
Whenever you build or modify game logic, mechanics, rendering, collisions, levels, UI/HUD, or progression, you MUST write and run a live automated playthrough bot against the real running game that plays a full loop and actively hunts for bugs:
1. Probe corners, walls, floor seams, boundaries, rapid inputs, and softlock states.
2. Run scripted telemetry checks for falling through floors/textures, collision clipping, out-of-bounds, NaN/console/WebGL/asset errors, and FPS drops.
3. Record video of the bot playthrough in a temp directory outside the repo, slice it into dense frames and contact sheets, and inspect the frames with view (when image viewing is supported) for visual/texture/HUD bugs.
4. Evaluate and verify difficulty and balance (win/loss rates, pacing, resource curves, progression fairness).
</gamedev_mode>`
	default:
		return ""
	}
}
