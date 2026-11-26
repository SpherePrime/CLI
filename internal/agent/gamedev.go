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
	return gameDevPromptBlockForSession(agentID, enabled, true, false)
}

func gameDevPromptBlockWithDelegation(agentID string, enabled, allowDelegation bool) string {
	return gameDevPromptBlockForSession(agentID, enabled, allowDelegation, false)
}

func gameDevPromptBlockForSession(agentID string, enabled, allowDelegation, strictVerify bool) string {
	if !enabled {
		return ""
	}
	delegateQAToReviewer := strictVerify && allowDelegation
	switch agentID {
	case config.AgentCode:
		if delegateQAToReviewer {
			return `

<gamedev_mode>
GameDev mode is enabled together with Strict Verify. Follow all GameDev engineering and asset standards upfront, and leave heavy GameDev QA execution to the independent review agent so checks are not duplicated:
First, read prime://skills/prime-gamedev/SKILL.md with view. Loading it exposes <skill_directory> (base helper scripts) and <gamedev_library> (persistent user-data library of accumulated game/engine scripts outside the repo).
1. Game engineering baseline upfront (apply across the whole project for your own game, or strictly to the mod's own items/weapons/structures/UI for a third-party game mod): draft a concrete balance plan before coding (speeds, jump reach, HP, damage, TTK, cooldowns, economy, difficulty curve, win/loss/restart reset), engineer airtight collisions from the start (visuals match hitboxes, axis-separated/swept checks, clamped/fixed physics dt <= 0.05s, reliable ground/seam/edge handling, 3D camera collision so there are zero floor/texture fall-throughs), ALWAYS include a crisp high-contrast crosshair/reticle with shot/hit/reload feedback whenever the game has any weapon or aimed attack, plus normalized diagonal movement, responsive game feel, telegraphed fair enemies, and a readable non-obstructive HUD.
2. Dedicated Asset Showcase scene (MANDATORY): build a clean standalone showcase/inspection scene (e.g. ?showcase=1 or a dedicated inspection view with neutral studio lighting) placing EVERY created AND downloaded visual element separately (characters, enemies, weapons, projectiles, reticles, props, structures, items, effects, or mod additions) with a simple camera/orbit selector hook (e.g. window.__SHOWCASE__.listElements() and focusElement(id, angle)) so the review agent's photo script can rapidly photograph every element from multiple sides.
3. Textures, sprites, and 3D models sourcing: unless the user asks you to craft textures/models yourself, search online and download ready high-quality assets (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists online, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). Every downloaded and custom asset MUST appear on the Showcase scene.
4. Collision debug hooks: expose clean collision/state telemetry hooks (and optional collision-only probe waypoints for large maps) so the review agent's live bot can run video-recorded collision checks cleanly without hijacking the user's mouse or keyboard.
5. Hand QA off to the independent review agent: verify that the project builds and launches cleanly, then DO NOT run the full multi-angle showcase photo sweep or the live collision-bot video recording yourself — the mandatory independent review agent performs both passes. Whenever the reviewer reports ANY visual asset defect from showcase photos or ANY collision/physics defect from the bot video, fix it in code/assets cleanly and return it for fresh re-review.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY requirement, not optional advice.
First, read prime://skills/prime-gamedev/SKILL.md with view. Loading it exposes <skill_directory> (base helper scripts) and <gamedev_library> (persistent user-data library of accumulated game/engine scripts outside the repo).
You MUST follow this workflow for ANY game or game mod (any engine, language, or framework):
1. Game engineering baseline upfront (apply across the whole project for your own game, or strictly to the mod's own items/weapons/structures/UI for a third-party game mod): draft a concrete balance plan before coding (speeds, jump reach, HP, damage, TTK, cooldowns, economy, difficulty curve, win/loss/restart reset), engineer airtight collisions from the start (visuals match hitboxes, axis-separated/swept checks, clamped/fixed physics dt <= 0.05s, reliable ground/seam/edge handling, 3D camera collision so there are zero floor/texture fall-throughs), ALWAYS include a crisp high-contrast crosshair/reticle with shot/hit/reload feedback whenever the game has any weapon or aimed attack, plus normalized diagonal movement, responsive game feel, telegraphed fair enemies, and a readable non-obstructive HUD.
2. Selective library & preset lookup: query built-in presets (<skill_directory>/scripts/presets/) and <gamedev_library>/index.json via library.mjs lookup filtered by --feature, --physics, --engine, or --game (read only the 1-2 relevant scripts to save context; never read unrelated folders).
3. Dedicated Asset Showcase scene & fast multi-angle photo check (for ALL visuals):
   - Build a clean standalone Showcase scene (e.g. ?showcase=1 or a dedicated inspection view with neutral lighting) containing EVERY created AND downloaded visual element standing separately: characters, enemies, weapons, items, props, structures, effects, and mod additions.
   - Run a fast automated photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine dump) that photographs every element from multiple sides (front, side, back, 3/4 for 3D; close-up and key states for 2D) without walking or teleporting around the gameplay map to inspect visuals.
   - Inspect all showcase photos/contact sheets with view. If ANY downloaded or custom model, weapon, sprite, or texture looks ugly, broken, misaligned, or mismatched, fix or remake it and re-shoot the showcase until clean.
4. Live gameplay bot & window-scoped video recording STRICTLY for collisions & physics (in the background, without hijacking the user's mouse or keyboard):
   - Video recording is used ONLY to verify collisions, physics, hitboxes, floor/texture fall-throughs, wall/corner clipping, stuck/softlock spots, out-of-bounds, 3D camera wall clipping, NaN physics, runtime/console/WebGL/asset errors, FPS, and core controls/balance — do NOT use gameplay video or map teleports to inspect how models/textures look (that belongs exclusively to the Showcase scene).
   - For browser games or web work when built-in browser tools (browser_navigate, browser_record_start, browser_record_stop, browser_run, browser_press_key, browser_evaluate, browser_screenshot) are available, do EVERYTHING completely inside Prime Desktop's built-in browser in the background tab (never launch external Chrome/Playwright and never take over the user's cursor). Use browser_record_start -> bot collision walkthrough with held keys (holdMs) & collision probes -> browser_record_stop -> extract_frames.mjs on the recorded video.
   - For CLI-only web fallback without built-in browser tools use web_bot_runner.mjs; for native/desktop games use native_bot_runner.mjs or window-scoped capture targeting ONLY the game's own window (never capture the full computer screen and never substitute static screenshots for video recording).
   - Visually inspect collision contact sheets/flagged frames with view for fall-throughs, clipping, hitbox mismatches, and stuck states.
5. Full game vs. mod scope:
   - When building your own standalone game, inspect all game elements on the Showcase scene and verify full-world collisions, gameplay loop, UI, and difficulty/balance.
   - When building a mod/plugin/addon for an existing third-party game, put only the mod's own elements on the Showcase scene and verify the mod's functionality and basic collisions — do NOT audit or try to fix unrelated bugs in the base game you did not write.
6. Textures, sprites, and 3D models sourcing & quality: unless the user asks you to craft textures/models yourself, search online and download ready high-quality assets (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists online, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). Always verify BOTH downloaded and custom assets via the Showcase multi-angle photos.
7. Fix every bug you find and re-verify: whenever Showcase photos reveal a visual/design defect or the collision bot video/telemetry reveals a collision, physics, softlock, or balance bug, fix it in code/assets and re-run the affected verification (fresh Showcase photos for visual fixes, fresh collision bot video run for collision/physics fixes) until clean.
8. Self-learning library: save new or improved reusable scripts into sorted <gamedev_library> folders (features/, physics/, engines/, elements/, games/) via library.mjs save with clear --feature/--physics/--summary/--tags so future games can reuse them immediately without deleting existing scripts, and clean up temporary video/frame dirs outside the repo.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY two-part verification requirement (visuals via Showcase scene, collisions via live bot video).
First, read prime://skills/prime-gamedev/SKILL.md with view. Use library.mjs lookup (or index.json) rather than reading unrelated scripts in <gamedev_library>.
Execute and inspect BOTH verification stages without hijacking the user's mouse or keyboard:
1. Confirm core game fundamentals are built cleanly (balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt, mandatory crosshair & combat feedback whenever weapons exist, clean HUD and full restart reset).
2. Stage A — Dedicated Showcase scene multi-angle photo audit (ALL visuals, custom AND downloaded):
   - Open the project's standalone Showcase scene (where all created and downloaded elements — characters, enemies, weapons, items, props, structures, effects, mod additions — stand separately) and run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine capture) photographing every element from multiple sides (front, side, back, 3/4 for 3D; close-ups/states for 2D).
   - Inspect all showcase photos/contact sheets with view (when image viewing is supported). Verify that every downloaded and custom texture, sprite, 3D model, and weapon looks clean, cohesive, properly scaled, and free of UV/seam/geometry defects. Do NOT teleport around the gameplay map to inspect how assets look.
3. Stage B — Live gameplay bot & window-scoped video recording STRICTLY for collisions & physics:
   - Run a real live walking bot in the gameplay scene inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run/press_key/evaluate) when browser tools exist, or via web_bot_runner.mjs / native_bot_runner.mjs targeting ONLY the target game window/viewport (NOT the full computer screen and NOT just standalone screenshots).
   - Slice the recorded video into dense frames and contact sheets (extract_frames.mjs in a temp directory outside the repo) and inspect them with view along with scripted telemetry STRICTLY for collisions and physics: floor/texture fall-throughs, wall/corner clipping, hitbox mismatches, seam snags, out-of-bounds, 3D camera wall clipping, stuck/softlock spots, NaN physics, console/WebGL/asset errors, and (for full games) difficulty/balance. Do NOT use the gameplay video to critique asset appearance (that is Stage A).
4. Respect full-game vs. mod scope: for a standalone game, verify all elements on Showcase and full-world collisions, gameplay loop, and difficulty/balance; for a mod/addon on a third-party game, verify only the mod's own elements on Showcase and the mod's own functionality/basic collisions without auditing unrelated base-game code.
5. Report missing Showcase scene or skipped multi-angle photos, missing live collision bot or video recording, external-browser usage when built-in browser tools exist, or ANY unfixed visual/asset defect from Showcase photos or collision/physics/softlock/difficulty defect from the bot video so it gets fixed and re-verified cleanly.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for a game or mod (on any engine or language) MUST reference prime://skills/prime-gamedev/SKILL.md and mandate:
1. Upfront game design & engineering baseline: a concrete balance plan before coding, airtight collisions & zero floor/texture fall-throughs (matching hitboxes, axis-separated/swept checks, fixed/clamped dt <= 0.05s, map bounds, 3D camera collision), mandatory high-contrast crosshair & combat feedback whenever any weapon/aimed attack exists, normalized controls/game feel, fair telegraphed enemies, clean state resets, and readable HUD (scoped to mod additions for third-party game mods).
2. Querying <gamedev_library> via library.mjs lookup (never reading all scripts blindly) to reuse relevant presets/scripts (including showcase_capture_probe.mjs and collision probes) and saving new scripts into sorted folders without deleting existing ones.
3. Asset sourcing & dedicated Showcase scene for visual QA: download ready textures/3D models online when not explicitly requested custom, or craft them directly when requested or unavailable online; place EVERY created and downloaded element (characters, weapons, props, structures, items, mod additions) separately on a dedicated Showcase scene and run a fast multi-angle photo script + view inspection to verify visual quality (never teleport around the gameplay map to inspect how assets look).
4. Live gameplay bot + video recording strictly for collisions & physics (in the background, without hijacking the user's mouse/keyboard): for browser games use Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run, browser_press_key with holdMs, browser_evaluate); for native games capture ONLY the target game window (never the full computer screen and never static screenshots alone); slice video into dense frames and contact sheets and check strictly for collisions, floor/texture fall-throughs, clipping, hitboxes, out-of-bounds, NaN physics, softlocks, runtime errors, and difficulty/balance.
5. A mandatory fix-and-rerun loop: any visual defect found on Showcase photos or collision/physics bug found on the bot video/telemetry must be fixed and re-verified cleanly.
</gamedev_mode>`
	case config.AgentGeneral:
		if delegateQAToReviewer {
			return `

<gamedev_mode>
GameDev mode is enabled together with Strict Verify (MANDATORY for every game or game mod regardless of engine or stack):
- Always start by reading prime://skills/prime-gamedev/SKILL.md with view to access <skill_directory> and <gamedev_library>.
- Keep roles strictly separated so GameDev QA is NOT duplicated:
  1. Implementation (you or code agents): design right from the start (concrete balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt <= 0.05s, mandatory crosshair & feedback whenever weapons exist, clean HUD/state resets), source textures/3D models from the web when not requested custom (or craft them directly when requested or unavailable online), build a dedicated Showcase scene containing EVERY created AND downloaded element standing separately (characters, weapons, items, props, structures, mod additions) with camera/angle focus hooks, expose collision/state debug hooks, and verify the project builds/starts cleanly. Do NOT run the full Showcase multi-angle photo sweep or live collision-bot video recording yourself before review.
  2. Independent review agent (MANDATORY): delegate the entire two-part GameDev QA to the independent review agent — (A) run the fast multi-angle photo script on the Showcase scene for every custom and downloaded element (including weapons) and inspect the photos with view; (B) run the live bot with built-in browser / window-scoped video recording (extract_frames.mjs) STRICTLY for collisions, physics, floor/texture fall-throughs, clipping, hitboxes, softlocks, runtime errors, and difficulty/balance (never using gameplay video or map teleports to judge asset visuals).
  3. Fix every visual defect from Showcase photos and every collision/physics bug reported by the review agent and send back for fresh re-review until VERDICT: APPROVED, then save reusable scripts to <gamedev_library> without deleting existing ones.
</gamedev_mode>`
		}
		if allowDelegation {
			return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
- Always start by reading prime://skills/prime-gamedev/SKILL.md with view to access <skill_directory> and <gamedev_library>.
- Require code agents to design right from the start (concrete balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt, mandatory crosshair & feedback whenever weapons exist, clean HUD/state resets), check <gamedev_library> selectively via library.mjs lookup (saving context, never reading unrelated folders), source textures/3D models from the web when not requested custom (or craft them themselves when requested or unavailable online), build a dedicated Showcase scene displaying EVERY created AND downloaded element separately (characters, weapons, props, structures, items, mod additions), run a fast multi-angle photo capture script on the Showcase scene and inspect the photos with view (fixing/remaking any ugly or broken asset), then build and run a live walking bot in the background (without hijacking the user's mouse/keyboard) inside Prime Desktop's built-in browser (browser_record_start/stop + browser_run/press_key/evaluate) or window-scoped capture targeting ONLY the game window (never the full computer screen and never just standalone screenshots), slice the video into dense frames and contact sheets, and verify STRICTLY collisions, physics, floor/texture fall-throughs, clipping, hitboxes, softlocks, runtime errors, and difficulty/balance (scoped to the whole game for standalone games, or to the mod's own features/elements/basic collisions for mods — never teleporting around the map or using gameplay video to judge asset visuals), FIX every bug found and re-verify cleanly, and save reusable scripts to the sorted library without deleting old ones.
- Require review agents (or your own verification when no reviewer is used) to confirm the Showcase multi-angle photo check of all custom and downloaded elements, built-in browser / window-scoped video recording of the live collision bot, dense frame inspection for collisions/physics, clean error/difficulty telemetry, and zero unfixed bugs.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
First read prime://skills/prime-gamedev/SKILL.md with view, then query <gamedev_library> selectively via library.mjs lookup (never reading unrelated folders) and carry out the full two-part QA loop directly:
1. Design and build with solid game fundamentals upfront: draft a concrete balance plan before coding, engineer airtight collisions and zero floor/texture fall-throughs (matching hitboxes, fixed/clamped physics dt <= 0.05s, map bounds, 3D camera collision), ALWAYS add a clear high-contrast crosshair and combat feedback whenever weapons or aimed attacks exist, plus clean controls, fair enemies, full state resets, and readable HUD (scoped strictly to mod additions for third-party game mods).
2. Textures, sprites, and 3D models sourcing + dedicated Showcase scene (for ALL visuals): unless the user asks you to make assets yourself, search and download ready quality assets online (or craft them yourself via generate_image/procedural code when requested or when no suitable asset exists online). Place EVERY created AND downloaded element separately on a dedicated Showcase scene (characters, enemies, weapons, items, props, structures, effects, mod additions), run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine capture) photographing every element from multiple sides, and inspect all photos with view. Fix or remake any asset or weapon that looks ugly, mismatched, or broken and re-shoot until clean.
3. Live automated walking bot & video recording STRICTLY for collisions & physics (in the background, without taking over the user's mouse/keyboard):
   - Do NOT teleport around the map or use gameplay video to inspect how models/textures look — gameplay video is strictly for verifying collisions, physics, hitboxes, floor/texture fall-throughs, wall/corner clipping, stuck/softlock spots, out-of-bounds, 3D camera wall clipping, NaN/console/WebGL/asset errors, FPS, and (for standalone games) difficulty/balance.
   - For browser games/web tasks when built-in browser tools (browser_navigate, browser_record_start, browser_record_stop, browser_run, browser_press_key, browser_evaluate, browser_screenshot) are available, perform everything completely inside Prime Desktop's built-in browser; otherwise use web_bot_runner.mjs or native_bot_runner.mjs. Record a real continuous video of ONLY the built-in browser tab or target game window/viewport into a temp directory outside the repo (never capture the full computer screen and never substitute static screenshots for video), slice the video into dense frames and contact sheets (extract_frames.mjs), and inspect them with view for collision/physics defects.
4. Fix every bug found in Showcase photos or collision video/telemetry and re-run the affected verification until clean.
5. Save new or improved reusable scripts into sorted <gamedev_library> folders via library.mjs save without deleting old ones.
</gamedev_mode>`
	default:
		return ""
	}
}
