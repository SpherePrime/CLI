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
GameDev mode is enabled. This is a MANDATORY, NON-NEGOTIABLE requirement for any game work.
First, read prime://skills/prime-gamedev/SKILL.md with view. This provides ready-to-use base scripts in <skill_directory>/scripts/ and a persistent sorted script library in <gamedev_library>.
To save context, NEVER read all scripts in <gamedev_library> blindly: run library.mjs lookup (or inspect index.json) for the current engine/game first, read only the 1-2 relevant scripts, and save any new or improved reusable scripts into the proper folder (engines/, games/, or elements/) without deleting existing ones.
Whenever you build, modify, or fix a game (on ANY engine, language, or framework — web, Canvas, WebGL, Godot, Unity, Pygame, Bevy, SDL, C++, Rust, terminal, or custom), you MUST figure out how to launch, control, and record it, and execute the full loop below:
1. Mandatory live in-game playthrough bot: write and run a real bot that actually enters the running game (clicks Start/Play through menus, controls the real player/game loop end-to-end) and actively hunts for bugs by probing corners, walls, floor seams, edges, rapid/conflicting inputs, and restart/respawn paths. Unit tests, static checks, or mocked simulations alone are NOT sufficient.
2. Mandatory real video recording + frame slicing + close-up element captures: select/target the game's own window or viewport directly (e.g. pass --window or auto-target the launched game window in native_bot_runner.mjs, or isolated browser viewport in web_bot_runner.mjs — never capture the full computer screen/desktop and never substitute static screenshots for video recording) into a temp directory outside the repo, slice that video into dense frames and contact sheets with extract_frames.mjs, AND capture close-up screenshots of individual created elements (player, enemies, items, HUD, menus, key objects) via captureElement/captureRegion or element_capture.mjs.
3. Visual frame & element inspection + scripted telemetry: inspect the extracted contact sheets, key/flagged frames, and close-up element screenshots with view (when image viewing is supported) to catch visual/rendering bugs, falling through floors/textures, missing/corrupt textures, z-fighting, camera clipping, and HUD overlaps, while also running scripted checks for floor/texture fall-throughs, collision clipping, out-of-bounds states, softlocks, NaN physics, console/WebGL/asset errors, FPS drops, and difficulty/balance fairness.
4. Textures, sprites, and 3D models sourcing & quality: unless the user asks you to craft textures/models yourself, search online and download ready high-quality assets (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists online, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). In all cases, inspect them in-game via element close-ups and video frames — if any downloaded or custom texture/model looks ugly, mismatched, or broken, fix or remake it yourself.
5. Fix every bug you find and re-verify: whenever the bot, scripted telemetry, frames, or element close-ups reveal ANY bug, visual glitch, ugly/broken texture or model, softlock, or balance problem, you MUST fix it in code/assets and re-run the live bot with a fresh video recording, frame extraction, element captures, and view inspection until the run is clean.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY verification requirement.
First, read prime://skills/prime-gamedev/SKILL.md with view. Use library.mjs lookup (or index.json) rather than reading unrelated scripts in <gamedev_library>.
Verification MUST execute and inspect a real live bug-hunting bot inside the actual game (regardless of engine/framework) with real video recording and close-up element captures:
1. Confirm the game has a live bot that actually enters the running game, completes a full playthrough loop, and stress-tests walls, corners, floor seams, boundaries, rapid inputs, and softlock spots.
2. Confirm a continuous video of the playthrough was actually recorded strictly from the target game window or game viewport (NOT the full computer screen and NOT just standalone screenshots) and sliced into dense frames and contact sheets (via web_bot_runner.mjs, native_bot_runner.mjs, or extract_frames.mjs in a temp directory outside the repo), AND confirm close-up screenshots of individual created elements (via element_capture.mjs or captureElement/captureRegion) were taken; inspect the contact sheets, key/flagged frames, and element close-ups with view (when image viewing is supported) for visual bugs, texture fall-throughs, missing/broken textures, z-fighting, camera clipping, and HUD overlaps.
3. Inspect scripted runtime/telemetry results to verify zero falls through floors/textures, collision clipping, out-of-bounds states, softlocks, NaN physics, console/WebGL/asset errors, or broken difficulty/balance.
4. Confirm textures, sprites, and 3D models look cohesive and clean in close-ups/frames (downloaded ready assets when not requested custom, or custom-made when requested or unavailable online, and fixed/remade if anything looks ugly or broken).
5. Report missing live in-game bots, missing video recording or element close-ups, skipped visual inspection, or ANY unfixed visual, texture, model, gameplay, collision, softlock, or difficulty defect so it gets fixed and re-verified with a fresh video run.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for a game (on any engine or language) MUST reference prime://skills/prime-gamedev/SKILL.md and mandate:
1. Querying <gamedev_library> via library.mjs lookup (never reading all scripts blindly) to reuse relevant engine/game/element scripts and saving new scripts into sorted folders without deleting existing ones.
2. Writing and running a live playthrough bot that actually enters the running game, completes a full loop, and actively hunts for collision, texture-fall-through, and softlock bugs.
3. Implementing real continuous video recording of ONLY the target game window or viewport (never the full computer screen and never static screenshots alone, adapting the capture method to the game's stack/engine), slicing the video into dense frames and contact sheets, capturing close-up screenshots of individual created elements, and visually inspecting both with view.
4. Scripted telemetry checks (falling through floors/textures, out-of-bounds, NaN physics, console/WebGL/asset errors, FPS) plus difficulty and balance verification.
5. Asset sourcing & visual polish: download ready textures/3D models online when not explicitly requested custom, or craft them directly when requested or unavailable online, and verify in close-ups/frames that none look ugly, mismatched, or broken.
6. A mandatory fix-and-rerun loop: any bug found in frames, element close-ups, or telemetry must be fixed and re-verified with a fresh bot video run.
</gamedev_mode>`
	case config.AgentGeneral, config.LegacyAgentCoder, "":
		if canDelegate {
			return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY, NON-NEGOTIABLE requirement for any game work.
Read prime://skills/prime-gamedev/SKILL.md with view and enforce the full GameDev QA loop across any game engine, language, or framework:
- Require code agents to check <gamedev_library> selectively via library.mjs lookup (saving context, never reading unrelated folders), use <skill_directory>/scripts/ base helpers, source textures/3D models from the web when not requested custom (or craft them themselves when requested or unavailable online, and fix/remake any asset that looks ugly or broken), build and run a live playthrough bot that enters the game and hunts for bugs, record a real continuous gameplay video capturing ONLY the target game window/viewport (never the full computer screen and never just standalone screenshots), slice it into dense frames and contact sheets, capture close-up screenshots of individual created elements, inspect frames and elements with view, run scripted collision/texture-fall-through/error/difficulty checks, FIX every bug found followed by a fresh video re-run, and save reusable scripts to the sorted library without deleting old ones.
- Require review agents (or your own <completion_check> when no reviewer is used) to verify the live in-game bot run, real window-scoped video recording, dense frame extraction, close-up element captures, visual inspection of frames/textures/models via view, clean collision/error/difficulty telemetry, and zero unfixed bugs.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY, NON-NEGOTIABLE requirement for any game work.
First, read prime://skills/prime-gamedev/SKILL.md with view to access the base scripts in <skill_directory>/scripts/ and the persistent sorted script library in <gamedev_library>.
To save context, query <gamedev_library> via library.mjs lookup (or index.json) first instead of reading unrelated scripts, and after verification save new/improved reusable scripts into engines/, games/, or elements/ without deleting existing ones.
Whenever you build, modify, or fix a game (on ANY engine, language, or framework — web, Godot, Unity, Pygame, Bevy, SDL, C++, Rust, terminal, or custom), you MUST determine how to launch, control, and record it, and execute the complete cycle:
1. Write and run a live automated playthrough bot that actually enters the running game (through menus/start), plays a full loop, and actively hunts for bugs across corners, walls, floor seams, boundaries, rapid inputs, and softlock states.
2. Record a real continuous video of ONLY the target game window or viewport into a temp directory outside the repo (never capture the full computer screen and never substitute static screenshots for video), slice the video into dense frames and contact sheets (web_bot_runner.mjs, native_bot_runner.mjs, or extract_frames.mjs), capture close-up screenshots of individual created elements (element_capture.mjs or captureElement/captureRegion), and inspect both frames and element close-ups with view (when image viewing is supported).
3. Run scripted telemetry checks for falling through floors/textures, collision clipping, out-of-bounds, NaN/console/WebGL/asset errors, FPS drops, and verify difficulty and balance fairness.
4. For textures, sprites, and 3D models: unless the user asks you to make them yourself, search and download ready quality assets online (or craft them yourself via generate_image/procedural code when requested or when no suitable asset exists online); visually check them in close-ups/frames and fix or remake any asset that looks ugly, mismatched, or broken.
5. Fix every bug found in frames, element close-ups, or telemetry and re-run the live bot with a new video recording and frame/element inspection until the playthrough is clean.
</gamedev_mode>`
	default:
		return ""
	}
}
