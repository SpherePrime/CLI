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
3. Textures, sprites, and 3D models sourcing: unless the user asks you to craft textures/models yourself, first check and reuse native engine/project asset libraries (especially in Unreal Engine: existing project Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans via Bridge or downloadable free assets), and search online to download ready high-quality assets (e.g. Fab, Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). Every engine-library, downloaded, and custom asset MUST appear on the Showcase scene.
4. Automated code-level collision tests (MANDATORY): keep collision geometry and physics resolution cleanly testable in code (single source of truth between rendered objects and colliders) so automated tests can deterministically verify every solid object has a real collider matching its visual bounds and blocks movement from all sides (4 sides in 2D / 6 faces in 3D), corners, floor/platform seams, high-speed swept motion, map bounds, and 3D camera walls — without teleporting the player around the map.
5. Hand QA off to the independent review agent: verify that the project builds and launches cleanly, then DO NOT run the full multi-angle showcase photo sweep, full collision test audit, or live gameplay video recording yourself — the mandatory independent review agent performs the full GameDev QA pass. Whenever the reviewer reports ANY visual asset defect from showcase photos, ANY collision test failure, or ANY gameplay/runtime defect, fix it in code/assets cleanly and return it for fresh re-review.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY requirement, not optional advice.
First, read prime://skills/prime-gamedev/SKILL.md with view. Loading it exposes <skill_directory> (base helper scripts) and <gamedev_library> (persistent user-data library of accumulated game/engine scripts outside the repo).
You MUST follow this workflow for ANY game or game mod (any engine, language, or framework):
1. Game engineering baseline upfront (apply across the whole project for your own game, or strictly to the mod's own items/weapons/structures/UI for a third-party game mod): draft a concrete balance plan before coding (speeds, jump reach, HP, damage, TTK, cooldowns, economy, difficulty curve, win/loss/restart reset), engineer airtight collisions from the start (visuals match hitboxes, axis-separated/swept checks, clamped/fixed physics dt <= 0.05s, reliable ground/seam/edge handling, 3D camera collision so there are zero floor/texture fall-throughs), ALWAYS include a crisp high-contrast crosshair/reticle with shot/hit/reload feedback whenever the game has any weapon or aimed attack, plus normalized diagonal movement, responsive game feel, telegraphed fair enemies, and a readable non-obstructive HUD.
2. Selective library & preset lookup: query built-in presets (<skill_directory>/scripts/presets/) and <gamedev_library>/index.json via library.mjs lookup filtered by --feature, --physics, --engine, or --game (read only the 1-2 relevant scripts to save context; never read unrelated folders).
3. Stage A — Dedicated Asset Showcase scene & fast multi-angle photo check (for ALL visuals):
   - Build a clean standalone Showcase scene (e.g. ?showcase=1 or a dedicated inspection view with neutral lighting) containing EVERY created AND downloaded visual element standing separately: characters, enemies, weapons, items, props, structures, effects, and mod additions.
   - Run a fast automated photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine dump) that photographs every element from multiple sides (front, side, back, 3/4 for 3D; close-up and key states for 2D).
   - Inspect all showcase photos/contact sheets with view. If ANY downloaded or custom model, weapon, sprite, or texture looks ugly, broken, misaligned, or mismatched, fix or remake it and re-shoot the showcase until clean.
4. Stage B — Automated code-level collision & physics tests (NO in-game teleporting):
   - Write and run real automated collision tests in code (using the project's test runner or presets/collision_suite_probe.mjs) instead of teleporting a bot around the gameplay map.
   - Deterministically verify in code that: (a) every solid object, wall, floor, prop, door, and item actually has a registered collider matching its rendered visual bounds (no hollow meshes or invisible walls); (b) collisions block penetration from ALL sides (all 4 sides in 2D / all 6 faces in 3D), corners, and floor/platform seams; (c) high-speed movement and projectiles cannot tunnel through thin floors/walls (clamped/swept dt <= 0.05s); (d) map bounds, out-of-bounds recovery, 3D camera wall collision, and NaN/Infinity guards all pass.
5. Stage C — Live gameplay bot & window-scoped video check (controls, gameplay loop, runtime & balance — in the background, without hijacking the user's mouse or keyboard):
   - Run a real live gameplay bot inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run, browser_press_key with holdMs, browser_evaluate) when browser tools are available, or web_bot_runner.mjs / native_bot_runner.mjs targeting ONLY the game's own window (never capture the full computer screen and never teleport around the map).
   - Slice the recorded video into frames/contact sheets (extract_frames.mjs in a temp directory outside the repo) and check with view + telemetry for real-time gameplay feel, menu -> play -> pause -> restart, weapon/crosshair firing, console/WebGL/asset errors, softlocks, FPS, and difficulty/balance — do NOT use gameplay video to judge individual asset appearance (that is Stage A).
6. Full game vs. mod scope:
   - When building your own standalone game, inspect all game elements on the Showcase scene, cover all world objects in code collision tests, and verify the gameplay loop, UI, and difficulty/balance.
   - When building a mod/plugin/addon for an existing third-party game, put only the mod's own elements on the Showcase scene and test only the mod's own colliders and functionality — do NOT audit or try to fix unrelated bugs in the base game you did not write.
7. Textures, sprites, and 3D models sourcing & quality: unless the user asks you to craft textures/models yourself, first check and reuse native engine/project libraries (especially in Unreal Engine: project Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans), and search online to download ready high-quality assets (e.g. Fab, Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt) into the project when needed; if the user asked you to make them OR no suitable asset exists, create them yourself (using generate_image, procedural shaders/scripts, or custom geometry). Always verify engine-library, downloaded, and custom assets via the Showcase multi-angle photos.
8. Fix every bug you find and re-verify: fix visual defects found on Showcase photos (re-shoot Showcase), fix collision flaws found by code collision tests (re-run collision test suite), and fix gameplay/runtime/balance bugs (re-run live bot video) until clean.
9. Self-learning library: save new or improved reusable scripts into sorted <gamedev_library> folders (features/, physics/, engines/, elements/, games/) via library.mjs save with clear --feature/--physics/--summary/--tags so future games can reuse them immediately without deleting existing scripts, and clean up temporary video/frame dirs outside the repo.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled. This is a MANDATORY three-stage verification requirement (Showcase visuals, code-level collision tests, live gameplay bot).
First, read prime://skills/prime-gamedev/SKILL.md with view. Use library.mjs lookup (or index.json) rather than reading unrelated scripts in <gamedev_library>.
Execute and inspect ALL verification stages without hijacking the user's mouse or keyboard:
1. Confirm core game fundamentals are built cleanly (balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt, mandatory crosshair & combat feedback whenever weapons exist, clean HUD and full restart reset).
2. Stage A — Dedicated Showcase scene multi-angle photo audit (ALL visuals, custom AND downloaded):
   - Open the project's standalone Showcase scene (where all created and downloaded elements — characters, enemies, weapons, items, props, structures, effects, mod additions — stand separately) and run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine capture) photographing every element from multiple sides (front, side, back, 3/4 for 3D; close-ups/states for 2D).
   - Inspect all showcase photos/contact sheets with view (when image viewing is supported). Verify that every downloaded and custom texture, sprite, 3D model, and weapon looks clean, cohesive, properly scaled, and free of UV/seam/geometry defects.
3. Stage B — Automated code-level collision & physics test verification (NO in-game teleporting):
   - Write or run real automated collision tests in code (using the project's test suite or presets/collision_suite_probe.mjs) — do NOT teleport a bot around the gameplay map to probe objects.
   - Verify in code that every solid object/wall/floor/prop/door/item actually has a collider matching its visual bounds, blocks movement from all 4 sides (2D) / all 6 faces (3D), handles corners/seams/stairs cleanly, prevents high-speed floor/wall tunneling, enforces map bounds and 3D camera collision, and never produces NaN physics.
4. Stage C — Live gameplay bot & window-scoped video check (runtime, controls, combat & balance):
   - Run a real live gameplay bot in the gameplay scene inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run/press_key/evaluate) when browser tools exist, or via web_bot_runner.mjs / native_bot_runner.mjs targeting ONLY the target game window/viewport (NOT the full computer screen and NOT teleporting around the map).
   - Slice the recorded video into frames/contact sheets (extract_frames.mjs in a temp directory outside the repo) and inspect with view + telemetry for real controls, menu/pause/restart flow, crosshair/weapon firing, console/WebGL/asset errors, softlocks, FPS, and (for full games) difficulty/balance. Do NOT use the gameplay video to critique asset appearance (that is Stage A).
5. Respect full-game vs. mod scope: for a standalone game, verify all elements on Showcase, full-world code collision tests, and gameplay/balance; for a mod/addon on a third-party game, verify only the mod's own elements on Showcase and the mod's own colliders/functionality without auditing unrelated base-game code.
6. Report missing Showcase scene or skipped multi-angle photos, missing or failing code-level collision tests, missing live gameplay bot or video recording, external-browser usage when built-in browser tools exist, or ANY unfixed visual/asset, collision, physics, softlock, or difficulty defect so it gets fixed and re-verified cleanly.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for a game or mod (on any engine or language) MUST reference prime://skills/prime-gamedev/SKILL.md and mandate:
1. Upfront game design & engineering baseline: a concrete balance plan before coding, airtight collisions & zero floor/texture fall-throughs (matching hitboxes, axis-separated/swept checks, fixed/clamped dt <= 0.05s, map bounds, 3D camera collision), mandatory high-contrast crosshair & combat feedback whenever any weapon/aimed attack exists, normalized controls/game feel, fair telegraphed enemies, clean state resets, and readable HUD (scoped to mod additions for third-party game mods).
2. Querying <gamedev_library> via library.mjs lookup (never reading all scripts blindly) to reuse relevant presets/scripts (including showcase_capture_probe.mjs and collision_suite_probe.mjs) and saving new scripts into sorted folders without deleting existing ones.
3. Stage A — Asset sourcing & dedicated Showcase scene for visual QA: reuse native engine/project libraries first (in Unreal Engine: project Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans) and download ready textures/3D models online when not explicitly requested custom, or craft them directly when requested or unavailable; place EVERY created, engine-library, and downloaded element (characters, weapons, props, structures, items, mod additions) separately on a dedicated Showcase scene and run a fast multi-angle photo script + view inspection to verify visual quality.
4. Stage B — Automated code-level collision tests (NO in-game teleporting): write deterministic code tests proving every solid object has a real collider matching its visual bounds and blocks movement from all 4 sides (2D) / 6 faces (3D), corners, seams, high-speed motion, map bounds, and 3D camera walls.
5. Stage C — Live gameplay bot + window-scoped video check (in the background, without hijacking the user's mouse/keyboard): run a real gameplay bot in Prime Desktop's built-in browser (or window-scoped capture for native games, never full screen) to verify controls, combat/crosshair, menu/restart, runtime errors, softlocks, and difficulty/balance.
6. A mandatory fix-and-rerun loop: any visual defect found on Showcase photos, collision test failure, or gameplay/balance bug must be fixed and re-verified cleanly.
</gamedev_mode>`
	case config.AgentGeneral:
		if delegateQAToReviewer {
			return `

<gamedev_mode>
GameDev mode is enabled together with Strict Verify (MANDATORY for every game or game mod regardless of engine or stack):
- Always start by reading prime://skills/prime-gamedev/SKILL.md with view to access <skill_directory> and <gamedev_library>.
- Keep roles strictly separated so GameDev QA is NOT duplicated:
  1. Implementation (you or code agents): design right from the start (concrete balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt <= 0.05s, mandatory crosshair & feedback whenever weapons exist, clean HUD/state resets), reuse native engine/project asset libraries first (in Unreal Engine: Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans) and source textures/3D models from the web when not requested custom (or craft them directly when requested or unavailable), build a dedicated Showcase scene containing EVERY created, engine-library, AND downloaded element standing separately (characters, weapons, items, props, structures, mod additions) with camera/angle focus hooks, keep collision geometry/physics cleanly exposed for automated code-level collision tests (no map teleport hacks), and verify the project builds/starts cleanly. Do NOT run the full Showcase multi-angle photo sweep, full collision audit, or live gameplay video recording yourself before review.
  2. Independent review agent (MANDATORY): delegate the entire three-stage GameDev QA to the independent review agent — (A) run the fast multi-angle photo script on the Showcase scene for every custom and downloaded element (including weapons) and inspect the photos with view; (B) run/verify deterministic code-level collision tests proving every solid object has a collider matching its visual bounds and blocks movement from all 4/6 sides, corners, seams, tunneling, bounds, and 3D camera walls (never teleporting around the map); (C) run the live gameplay bot with built-in browser / window-scoped video recording (extract_frames.mjs) to verify controls, combat/crosshair, runtime errors, softlocks, and difficulty/balance.
  3. Fix every visual defect from Showcase photos, every collision test failure, and every gameplay/runtime bug reported by the review agent and send back for fresh re-review until VERDICT: APPROVED, then save reusable scripts to <gamedev_library> without deleting existing ones.
</gamedev_mode>`
		}
		if allowDelegation {
			return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
- Always start by reading prime://skills/prime-gamedev/SKILL.md with view to access <skill_directory> and <gamedev_library>.
- Require code agents to design right from the start (concrete balance plan, airtight collisions with zero floor/texture fall-throughs, fixed/clamped physics dt, mandatory crosshair & feedback whenever weapons exist, clean HUD/state resets), check <gamedev_library> selectively via library.mjs lookup (saving context, never reading unrelated folders), reuse native engine/project libraries first (in Unreal Engine: Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans) and source textures/3D models from the web when not requested custom (or craft them themselves when requested or unavailable), and run all three QA stages: (A) build a dedicated Showcase scene displaying EVERY created AND downloaded element separately (characters, weapons, props, structures, items, mod additions), run a fast multi-angle photo capture script and inspect with view (fixing/remaking any ugly or broken asset); (B) write and run deterministic code-level collision tests verifying every solid object has a real collider matching its visual bounds and blocks movement from all 4/6 sides, corners, seams, high-speed motion, map bounds, and 3D camera walls (NO in-game teleporting around the map); (C) run a live gameplay bot in the background (without hijacking the user's mouse/keyboard) inside Prime Desktop's built-in browser (browser_record_start/stop + browser_run/press_key/evaluate) or window-scoped capture targeting ONLY the game window to verify controls, combat/crosshair, runtime errors, softlocks, and difficulty/balance (scoped to the whole game for standalone games, or to the mod's own features/elements/colliders for mods), FIX every bug found and re-verify cleanly, and save reusable scripts to the sorted library without deleting old ones.
- Require review agents (or your own verification when no reviewer is used) to confirm the Showcase multi-angle photo check of all custom and downloaded elements, passing code-level collision tests across all sides/objects, built-in browser / window-scoped video recording of the live gameplay bot, clean error/difficulty telemetry, and zero unfixed bugs.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod regardless of engine or stack):
First read prime://skills/prime-gamedev/SKILL.md with view, then query <gamedev_library> selectively via library.mjs lookup (never reading unrelated folders) and carry out the full three-stage QA loop directly:
1. Design and build with solid game fundamentals upfront: draft a concrete balance plan before coding, engineer airtight collisions and zero floor/texture fall-throughs (matching hitboxes, fixed/clamped physics dt <= 0.05s, map bounds, 3D camera collision), ALWAYS add a clear high-contrast crosshair and combat feedback whenever weapons or aimed attacks exist, plus clean controls, fair enemies, full state resets, and readable HUD (scoped strictly to mod additions for third-party game mods).
2. Stage A — Textures, sprites, and 3D models sourcing + dedicated Showcase scene (for ALL visuals): unless the user asks you to make assets yourself, first reuse native engine/project libraries (in Unreal Engine: Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans) and search/download ready quality assets online (or craft them yourself via generate_image/procedural code when requested or when no suitable asset exists). Place EVERY created, engine-library, AND downloaded element separately on a dedicated Showcase scene (characters, enemies, weapons, items, props, structures, effects, mod additions), run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine capture) photographing every element from multiple sides, and inspect all photos with view. Fix or remake any asset or weapon that looks ugly, mismatched, or broken and re-shoot until clean.
3. Stage B — Automated code-level collision & physics tests (NO in-game teleporting): write and run real automated collision tests in code (using the project's test runner or presets/collision_suite_probe.mjs) instead of teleporting a bot around the gameplay map. Verify deterministically that every solid object/wall/floor/prop/door/item has a real collider matching its visual bounds, blocks movement from ALL sides (4 sides in 2D / 6 faces in 3D), handles corners/seams cleanly, prevents high-speed tunneling, enforces map bounds and 3D camera collision, and produces zero NaN physics.
4. Stage C — Live gameplay bot & window-scoped video check (controls, gameplay loop, runtime & balance — in the background, without taking over the user's mouse/keyboard): for browser games/web tasks when built-in browser tools (browser_navigate, browser_record_start, browser_record_stop, browser_run, browser_press_key, browser_evaluate, browser_screenshot) are available, perform everything completely inside Prime Desktop's built-in browser; otherwise use web_bot_runner.mjs or native_bot_runner.mjs. Record a real continuous video of ONLY the built-in browser tab or target game window/viewport into a temp directory outside the repo (never capture the full computer screen and never teleport around the map), slice the video into frames/contact sheets (extract_frames.mjs), and verify controls, combat/crosshair, menu/restart, runtime/console/WebGL/asset errors, softlocks, FPS, and difficulty/balance.
5. Fix every bug found in Showcase photos, code collision tests, or live gameplay video/telemetry and re-run the affected verification until clean.
6. Save new or improved reusable scripts into sorted <gamedev_library> folders via library.mjs save without deleting old ones.
</gamedev_mode>`
	default:
		return ""
	}
}
