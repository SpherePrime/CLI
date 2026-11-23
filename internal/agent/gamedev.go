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
	enabled, ok := ctx.Value(gameDevContextKey{}).(bool)
	return enabled, ok
}

func gameDevPromptBlock(agentID string, enabled bool) string {
	return gameDevPromptBlockWithDelegation(agentID, enabled, true)
}

func gameDevPromptBlockWithDelegation(agentID string, enabled, allowDelegation bool) string {
	if !enabled {
		return ""
	}
	switch agentID {
	case config.AgentCode:
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY requirement, not optional advice.
First, read prime://skills/prime-gamedev/SKILL.md with view. Loading it exposes <skill_directory> (base helper scripts) and <gamedev_library> (persistent user-data library of accumulated game/engine scripts outside the repo).
You MUST follow this workflow for ANY game or game mod (any engine, language, or framework):
1. Selective library lookup: query <gamedev_library>/index.json via library.mjs lookup for matching engine/game/element scripts (read only the 1-2 relevant scripts to save context; never read unrelated folders).
2. Mandatory live walking & bug-hunting bot (in the background, without hijacking the user's mouse or keyboard): write and run a real automated playthrough bot that actually enters gameplay and walks/moves thoroughly through the world (all directions, jumps, dashes, doors/zones, pressing into walls, corners, floor seams, edges, triggers, enemies/items, HUD, pause/restart), checking collisions, falling through floors/textures, out-of-bounds, NaN physics, softlocks, runtime/console/WebGL/asset errors, FPS, and difficulty/balance.
3. Mandatory built-in browser for web games & window-scoped video recording:
   - For browser games or web work when built-in browser tools (browser_navigate, browser_record_start, browser_record_stop, browser_run, browser_press_key, browser_evaluate, browser_screenshot) are available, do EVERYTHING completely inside Prime Desktop's built-in browser in the background tab (never launch external Chrome/Playwright and never take over the user's cursor). Use browser_record_start -> bot walkthrough with held keys (holdMs) & click coordinates -> browser_record_stop -> extract_frames.mjs on the recorded video, plus browser_screenshot for close-ups.
   - For CLI-only web fallback without built-in browser tools use web_bot_runner.mjs; for native/desktop games use native_bot_runner.mjs or window-scoped capture targeting ONLY the game's own window (never capture the full computer screen and never substitute static screenshots for video recording).
   - Also capture close-up screenshots of individual created elements (player, enemies, items, HUD, menus, mod additions) via browser_screenshot or element_capture.mjs / captureElement / captureRegion, and visually inspect both contact sheets/frames and element close-ups with view.
4. Full game vs. mod scope:
   - When building your own standalone game, inspect and verify the entire game thoroughly (world geometry, textures, 3D models, collisions, gameplay loop, UI, and difficulty/balance).
   - When building a mod/plugin/addon for an existing third-party game, focus verification on the mod's own functionality, added/modified textures/models/UI, and basic collisions/interactions — do NOT audit or try to fix unrelated bugs in the base game you did not write.
5. Textures, sprites, and 3D models sourcing & quality: unless the user asks you to craft textures/models yourself, search online and download ready high-quality assets (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists online, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). In all cases, inspect them in-game via element close-ups and video frames — if any downloaded or custom texture/model looks ugly, mismatched, or broken, fix or remake it yourself.
6. Fix every bug you find and re-verify: whenever the bot, scripted telemetry, frames, or element close-ups reveal ANY bug, visual glitch, ugly/broken texture or model, collision defect, softlock, or balance problem, you MUST fix it in code/assets and re-run the live bot with a fresh video recording, frame extraction, element captures, and view inspection until the run is clean.
7. Self-learning library: save new or improved reusable scripts into sorted <gamedev_library> folders (engines/, games/, elements/) via library.mjs save without deleting existing scripts, and clean up temporary video/frame dirs outside the repo.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY verification requirement.
First, read prime://skills/prime-gamedev/SKILL.md with view. Use library.mjs lookup (or index.json) rather than reading unrelated scripts in <gamedev_library>.
Verification MUST execute and inspect a real live walking bug-hunting bot inside the actual game (regardless of engine/framework) without hijacking the user's mouse or keyboard:
1. Confirm the game has a live bot that actually enters gameplay and walks/moves thoroughly through the world (all directions, zones, walls, corners, floor seams, boundaries, interactions, rapid inputs, and softlock spots). For browser games with built-in browser tools available, confirm everything ran inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run/press_key/evaluate) rather than external browsers.
2. Confirm a continuous video of the playthrough was actually recorded strictly from the built-in browser tab or target game window/viewport (NOT the full computer screen and NOT just standalone screenshots) and sliced into dense frames and contact sheets (via browser_record_start/stop + extract_frames.mjs, web_bot_runner.mjs, or native_bot_runner.mjs in a temp directory outside the repo), AND confirm close-up screenshots of individual created elements were taken; inspect the contact sheets, key/flagged frames, and element close-ups with view (when image viewing is supported) for visual bugs, texture fall-throughs, missing/broken textures, z-fighting, camera clipping, and HUD overlaps.
3. Respect full-game vs. mod scope: for a standalone game, verify full-world collisions, textures, models, gameplay loop, and difficulty/balance; for a mod/addon on a third-party game, verify the mod's own functionality, visuals, and basic collisions without auditing unrelated base-game code.
4. Confirm textures, sprites, and 3D models look cohesive and clean in close-ups/frames (downloaded ready assets when not requested custom, or custom-made when requested or unavailable online, and fixed/remade if anything looks ugly or broken).
5. Inspect scripted runtime/telemetry results to verify zero falls through floors/textures, collision clipping, out-of-bounds states, softlocks, NaN physics, console/WebGL/asset errors, or broken difficulty/balance.
6. Report missing live walking bots, external-browser usage when built-in browser tools exist, missing video recording or element close-ups, skipped visual inspection, or ANY unfixed visual, texture, model, gameplay, collision, softlock, or difficulty defect so it gets fixed and re-verified with a fresh video run.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for a game or mod (on any engine or language) MUST reference prime://skills/prime-gamedev/SKILL.md and mandate:
1. Querying <gamedev_library> via library.mjs lookup (never reading all scripts blindly) to reuse relevant engine/game/element scripts and saving new scripts into sorted folders without deleting existing ones.
2. Writing and running a live walking playthrough bot in the background (without hijacking the user's mouse/keyboard) that actually enters gameplay, moves across levels/zones, and actively hunts for collision, texture-fall-through, and softlock bugs (scoped to the full game for standalone games, or to the mod's own features/visuals/basic collisions for third-party game mods).
3. For browser games/web tasks when built-in browser tools exist, doing everything completely inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run, browser_press_key with holdMs, browser_evaluate, browser_screenshot); for native games, capturing ONLY the target game window (never the full computer screen and never static screenshots alone), slicing video into dense frames and contact sheets, capturing close-up screenshots of individual created elements, and visually inspecting both with view.
4. Scripted telemetry checks (falling through floors/textures, out-of-bounds, NaN physics, console/WebGL/asset errors, FPS) plus difficulty and balance verification (for standalone games).
5. Asset sourcing & visual polish: download ready textures/3D models online when not explicitly requested custom, or craft them directly when requested or unavailable online, and verify in close-ups/frames that none look ugly, mismatched, or broken.
6. A mandatory fix-and-rerun loop: any bug found in frames, element close-ups, or telemetry must be fixed and re-verified with a fresh bot video run.
</gamedev_mode>`
	case config.AgentGeneral:
		if allowDelegation {
			return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
- Always start by reading prime://skills/prime-gamedev/SKILL.md with view to access <skill_directory> and <gamedev_library>.
- Require code agents to check <gamedev_library> selectively via library.mjs lookup (saving context, never reading unrelated folders), use <skill_directory>/scripts/ base helpers, source textures/3D models from the web when not requested custom (or craft them themselves when requested or unavailable online, and fix/remake any asset that looks ugly or broken), build and run a live walking playthrough bot in the background (without hijacking the user's mouse/keyboard) that enters the game and hunts for bugs, do browser games completely inside Prime Desktop's built-in browser when browser_* tools are available (browser_record_start/stop + browser_run/press_key/evaluate), record a real continuous gameplay video capturing ONLY the built-in browser tab or target game window/viewport (never the full computer screen and never just standalone screenshots), slice it into dense frames and contact sheets, capture close-up screenshots of individual created elements, inspect frames and elements with view, run scripted collision/texture-fall-through/error/difficulty checks (scoped to the whole game for standalone games, or to the mod's own features/visuals/basic collisions for mods), FIX every bug found followed by a fresh video re-run, and save reusable scripts to the sorted library without deleting old ones.
- Require review agents (or your own <completion_check> when no reviewer is used) to verify the live walking bot run, built-in browser execution for web games, real window-scoped video recording, dense frame extraction, close-up element captures, visual inspection of frames/textures/models via view, clean collision/error/difficulty telemetry, and zero unfixed bugs.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
First read prime://skills/prime-gamedev/SKILL.md with view, then query <gamedev_library> selectively via library.mjs lookup (never reading unrelated folders) and carry out the full QA loop directly:
1. Write and run a live automated walking playthrough bot in the background (without taking over the user's mouse/keyboard) that actually enters the running game, moves through levels/zones, probes walls/corners/floor seams/boundaries, tests interactions and softlocks, and hunts for bugs (scoped to the whole game for standalone games, or to the mod's own features/visuals/basic collisions for third-party game mods).
2. For browser games/web tasks when built-in browser tools (browser_navigate, browser_record_start, browser_record_stop, browser_run, browser_press_key, browser_evaluate, browser_screenshot) are available, perform everything completely inside Prime Desktop's built-in browser; otherwise use web_bot_runner.mjs or native_bot_runner.mjs. Record a real continuous video of ONLY the built-in browser tab or target game window/viewport into a temp directory outside the repo (never capture the full computer screen and never substitute static screenshots for video), slice the video into dense frames and contact sheets (extract_frames.mjs), capture close-up screenshots of individual created elements (browser_screenshot or element_capture.mjs / captureElement / captureRegion), and inspect both frames and element close-ups with view (when image viewing is supported).
3. Run scripted telemetry checks for falling through floors/textures, collision clipping, out-of-bounds, NaN/console/WebGL/asset errors, FPS drops, and verify difficulty and balance fairness (for standalone games).
4. For textures, sprites, and 3D models: unless the user asks you to make them yourself, search and download ready quality assets online (or craft them yourself via generate_image/procedural code when requested or when no suitable asset exists online); visually check them in close-ups/frames and fix or remake any asset that looks ugly, mismatched, or broken.
5. Fix every bug found in frames, element close-ups, or telemetry and re-run the live bot with a new video recording and frame/element inspection until the playthrough is clean.
6. Save new or improved reusable scripts into sorted <gamedev_library> folders via library.mjs save without deleting old ones.
</gamedev_mode>`
	default:
		return ""
	}
}
