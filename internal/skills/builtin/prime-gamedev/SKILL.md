---
name: prime-gamedev
description: Game design/engineering baseline (balance plan, ideal collisions, weapons & crosshair, game feel, HUD), live playthrough and fast-teleport bot testing, built-in browser / window-scoped video recording, video-to-frames visual bug hunting, close-up element capture, texture/model sourcing, and sorted script library for games and mods.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev Engineering & QA

Design and build games right from the start (concrete balance plan, airtight collisions with zero texture fall-throughs, mandatory crosshair and feedback whenever weapons exist, clean controls, fair enemies, readable HUD), then verify every game or mod by running a real live bot (using both real-time gameplay and fast teleport probes to structures/items) under continuous video recording without taking over the user's mouse or keyboard, slicing the video into dense frames, capturing close-up screenshots of created elements, inspecting everything with `view`, and fixing every bug found until a fresh video run passes cleanly.

When this file is loaded with `view`, Prime exposes `<skill_directory>` (ready-to-run base scripts under `scripts/`, updated on `prime update`) and `<gamedev_library>` (persistent user-data library of accumulated game/engine scripts outside the repo, never overwritten on updates).

## Game Design & Engineering Baseline (Build Right From the Start)

Follow these fundamentals proactively while designing and coding (for a third-party game mod, apply them strictly to the mod's own elements, weapons, structures, collisions, UI, and balance):

1. **Balance Plan & State Architecture Before Coding:**
   - Draft a concrete balance plan before writing gameplay code: movement speed, jump height/reach, player and enemy HP, damage, time-to-kill (TTK), cooldowns, economy/pickups, difficulty curve across levels/waves, and win/loss conditions.
   - Implement a clean state lifecycle (`Menu -> Gameplay -> Pause -> Win/Loss -> Restart`) where Restart completely resets timers, enemies, projectiles, physics velocity/positions, and score.
   - Use a fixed physics timestep (or clamp `dt <= 0.05s`) so frame spikes never tunnel entities through floors or walls.
2. **Ideal Collisions & Zero Texture Fall-Throughs:**
   - Maintain a single source of truth between visuals and hitboxes: colliders must accurately match rendered sprites/meshes (no invisible walls, hollow solid-looking meshes, or offset hitboxes).
   - Resolve collisions cleanly per axis (X/Y/Z) and use continuous/swept checks for fast actors and projectiles to prevent floor fall-throughs, corner snagging, and wall tunneling.
   - Implement reliable ground checks, clean handling of tile/platform seams, stairs, ceilings, and hard map boundaries (plus out-of-bounds kill-zone/recovery).
   - In 3D / third-person views, add camera collision so the camera never clips inside walls or geometry.
3. **Weapons, Mandatory Crosshair & Combat (Whenever Combat Exists):**
   - **Crosshair is mandatory:** if the game includes any gun, bow, throwable, turret, or aimed spell, include a crisp, high-contrast crosshair/reticle (screen center for 3D/FPS, cursor-aligned for 2D/top-down) with visual feedback for firing/spread, hitmarker on hit, and reload state.
   - Align the shot/ray origin with the crosshair; provide visible tracers/projectiles, muzzle flash, recoil/kick, cooldown, ammo/reload loop, and fair hitboxes.
   - For melee combat, provide a readable swing arc/telegraph, knockback, and brief invulnerability frames (i-frames) after taking damage so the player is never deleted in a single frame.
4. **Controls & Game Feel:**
   - Normalize diagonal movement so diagonal speed is not faster; keep input snappy; add coyote time and jump buffering for platformers.
   - Give tangible feedback for actions: hit flashes, particles, subtle screen shake, animations, and clear pickup/damage/death cues.
5. **Fair Enemies & Zero Softlocks:**
   - Telegraph enemy attacks with visible wind-up animations/indicators; avoid unfair instant one-shots or spawning enemies directly on top of the player.
   - Guarantee every room/level is completable with no permanent softlocks or inescapable pits.
6. **Visual Readability, Assets & Clean HUD:**
   - Maintain clear figure-ground contrast so player, enemies, projectiles, and interactive items stand out immediately against floors and walls.
   - Prevent z-fighting (coplanar overlapping surfaces), stretched UVs, and pitch-black unlit areas.
   - Keep HUD clean and non-obstructive (HP, weapon/ammo, goal/score) with concise control hints on start/pause screens.

## Core QA Rules

1. **Keep temporary videos and frames outside the repo** (the helper scripts and `browser_record_start` default to `os.tmpdir()`) so `git status` stays clean.
2. **Never rely on unit tests or standalone screenshots alone.** Launch the real game, drive it with a live bot that actually enters gameplay through menus/start screens, and record a continuous video of the playthrough.
3. **Never hijack the user's mouse or keyboard and never record the full computer screen.**
   - **Browser games & web tasks (when `browser_*` built-in browser tools are available):** do EVERYTHING completely inside Prime Desktop's built-in browser in the background (`browser_navigate`, `browser_record_start`, `browser_run`, `browser_press_key`, `browser_evaluate`, `browser_screenshot`, `browser_record_stop`). Do NOT open external Chrome/Playwright when built-in browser tools are available, and never move the user's physical cursor.
   - **Native/desktop games (or CLI-only web fallback without `browser_*`):** record strictly the target game window or isolated headless browser viewport (`native_bot_runner.mjs` / `web_bot_runner.mjs`), never the full computer desktop.
4. **Own Game vs. Mod Scope:**
   - **Full game you are building:** verify the whole game end-to-end — core gameplay loop, weapons/crosshair, structures, items, walls, corners, floor seams, edges, enemies, textures, 3D models, collisions, UI/HUD, win/loss/restart states, and difficulty balance, fixing every bug found.
   - **Mod / plugin / addon for an existing third-party game:** verify **specifically the mod's own functionality** — check how the mod's added/modified structures, items, weapons/crosshair, textures, models, UI, mechanics, and basic collisions look and behave in-game. Do NOT spend time auditing or trying to fix bugs in the unrelated base game that you did not build.
5. **Fast Teleport Collision & Visual Inspection + Real Gameplay (All Engines & Platforms):**
   - Expose a debug/test teleport and camera-aim hook during testing (for example `window.__GAME_DEBUG__.teleport(x, y, z)` and `lookAt(x, y, z)` in web games, or a bot debug command/IPC in native games).
   - Instead of slowly walking across the entire map just to reach every distant structure or item, let the bot **teleport directly next to each target structure, item, wall, corner, platform, door, enemy, or mod object**, walk/probe briefly on the spot (pressing into surfaces/floors/corners to confirm zero fall-throughs, clipping, or getting stuck), and aim the camera directly at the structure/item so its visuals (textures, model, scale, seams, lighting) are clearly captured on video and/or close-up screenshots for `view` inspection.
   - Combine this fast teleport sweep with real-time live gameplay checks (start menu, real movement/controls, aiming/shooting with crosshair if weapons exist, combat, interactions, pause/restart, and difficulty balance).
6. **Capture close-ups of individual created elements in addition to the playthrough video.** Shoot focused close-up screenshots of key elements you built or changed (player character, weapons/crosshair, enemies, structures, items, effects, HUD/UI panels, menus, mod items/blocks) and inspect them with `view`.
7. **Sourcing & polishing textures and 3D models:**
   - Unless the user asked you to craft textures/models yourself, first search online (`web_search`, `agentic_fetch`, `fetch`) and download ready, license-compatible assets via `download` (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt in `.png`, `.webp`, `.glb`, `.gltf`, `.obj`) into the project's asset folder.
   - Craft them yourself (via `generate_image`, procedural shaders/canvas scripts, or asset builders) whenever the user explicitly asks you to make them, or when no suitable asset exists online.
   - Always inspect integrated assets in-game via `view` (video frames and element close-ups). If any downloaded or custom texture/model looks ugly, cheap, mismatched in style/scale, or has UV/seam/rendering defects, fix or remake it yourself.
8. **Reuse built-in presets & save context with `<gamedev_library>`.** Built-in reusable presets already ship under `<skill_directory>/scripts/presets/` (`teleport_collision_probe.mjs`, `weapon_crosshair_probe.mjs`, `physics_bounds_probe.mjs`, `balance_and_state_probe.mjs`) and are automatically indexed by `library.mjs lookup`. Never read all saved scripts blindly: always query `library.mjs lookup` by `--feature`, `--physics`, `--engine`, or `--game` first, read only the 1–2 matching scripts, and after a successful run save new or improved reusable scripts into the proper sorted folder (`features/`, `physics/`, `engines/`, `elements/`, `games/`) with a clear purpose/summary so future games can reuse them immediately without deleting existing ones.
9. **Fix every bug found and re-run.** Whenever frames, element close-ups, or telemetry reveal any bug, fix it in code/assets and repeat the full live bot + video + frame/element inspection loop until clean.

## Step 1: Lookup Relevant Presets & Library Scripts (Context-Efficient)

Before writing a bot or probe script from scratch, query both built-in presets (`<skill_directory>/scripts/presets/`) and `<gamedev_library>/index.json` via `library.mjs` filtered by feature, physics topic, engine, or game slug:

```bash
node "<skill_directory>/scripts/library.mjs" lookup --feature "<feature>" --physics "<topic>" --engine "<engine>" --game "<game-slug>" --limit 6
```

Read **only** the 1–2 matching scripts returned by `lookup` (for example `presets/teleport_collision_probe.mjs`, `presets/weapon_crosshair_probe.mjs`, `features/<feature>/...`, or `physics/<topic>/...`), plus `<skill_directory>/scripts/` base helpers as needed. Never read unrelated folders.

## Step 2: Run Live Bot (Real Gameplay + Fast Teleport Probes) + Video + Element Close-Ups

### A. Browser Games & Web Apps in Prime Desktop (Preferred — Built-In Browser Only)

When built-in browser tools (`mcp_browser_*`) are available, run the entire test inside the isolated built-in browser tab without touching the user's mouse or launching external Playwright:
1. Start the game dev server in the background (if needed) and open the game URL in the built-in browser with `browser_navigate` (using this chat's tab).
2. Start background video recording of that tab with `browser_record_start` (`fps: 10`).
3. Drive a live bot inside the tab using:
   - `browser_run` with `click{selector|text|x,y}` (to enter Start/Play menus or canvas buttons), `press_key{key,holdMs}` (real held movement/action keys like `ArrowRight`/`KeyD`/`KeyW`/`Space` for 300–1000ms), and `evaluate{expression}` (installing console/WebGL/asset-error hooks, invoking `teleport(x, y, z)` / `lookAt(x, y, z)` to jump straight to each structure/item/zone for quick local collision & visual checks, and reading telemetry such as coordinates, grounded state, collisions, floor fall-throughs, `NaN`, stuck/softlock checks, FPS, and difficulty samples).
   - `browser_press_key` (`holdMs`) and `browser_evaluate` as needed for multi-stage live gameplay + teleport checks across structures, items, and combat.
   - `browser_screenshot` (`clip`, `ref`, `zoom: 2`) for close-up captures of created characters, weapons/crosshair, structures, textures, models, and HUD/UI elements.
4. Stop recording with `browser_record_stop` to get `videoPath` (`playthrough.mp4`) and `framesDir`.
5. Slice `videoPath` (or `framesDir` if ffmpeg is absent) into dense contact sheets and `manifest.json` using `extract_frames.mjs`:

```bash
node "<skill_directory>/scripts/extract_frames.mjs" --video "<videoPath>" --fps 4 --max-frames 120
```

### B. Web Fallback in CLI Mode (No Built-In Browser)

Only when `browser_*` tools are unavailable, run `web_bot_runner.mjs` (headless Chromium, never touches user's mouse):

```bash
node "<skill_directory>/scripts/web_bot_runner.mjs" \
  --url "http://127.0.0.1:5173" \
  --bot "./path/to/bot.mjs" \
  --fps 4 \
  --max-frames 120
```

Inside `--bot` (`export async function runBot({ page, canvas, helpers })`), combine real gameplay inputs (`helpers.holdKey`, `helpers.tapKey`, `helpers.clickCanvas`) with fast teleport probes to structures/items (`page.evaluate(...)`), plus `helpers.captureElement`, `helpers.captureRegion`, `helpers.captureEventFrame`, `helpers.readGameState`, `helpers.recordDifficultySample`.

### C. Native / Desktop / Engine Games (Godot, Unity, Pygame, Bevy, SDL, Rust, C++, Mods)

Use `native_bot_runner.mjs` (target the game window via `--window "<title>"` or auto-detected PID, recording **strictly that game window**, never the whole desktop):

```bash
node "<skill_directory>/scripts/native_bot_runner.mjs" \
  --cmd "<launch command>" \
  --cwd "<game dir>" \
  --window "<Window Title>" \
  --bot-cmd "<bot driver command>" \
  --fps 4 \
  --max-frames 120 \
  --element-crops "player:400,260,220,220;hud:0,0,460,90"
```

Use a debug/test teleport hook in the game/mod when helpful so the bot can jump quickly to target structures/items to probe local collisions and frame their visuals on video. Slice any recorded video or engine-dumped PNG frames with `extract_frames.mjs`, and crop element close-ups with `element_capture.mjs --image <frame.png> --crops "..."`.

## Step 3: Visual Inspection (`view`) + Telemetry + Difficulty

1. Read `report.json` (if any) and `frames/manifest.json`.
2. Inspect **all** generated `contactSheets` (`sheet_*.png`) with `view` chronologically, then inspect individual `frames/frame_XXXX.png` for anomalies, transitions, or suspected bugs.
3. Inspect **all** `elementCaptures` / `elements/*.png` / `browser_screenshot` close-ups with `view` to verify each created element, structure, item, weapon/crosshair, texture, model, and HUD component looks crisp and defect-free.
4. Hunt specifically for:
   - Falling through floors/textures, wall/geometry clipping, mismatched hitboxes, out-of-bounds movement, stuck/softlock spots
   - Missing, checkerboard, stretched, seam-broken, or flickering (z-fighting) textures and ugly/broken 3D models
   - Missing/misaligned crosshair or broken weapon feedback (when weapons exist)
   - Broken shaders/lighting, black/frozen frames, particle leaks, HUD overlap/clipping, broken animations
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, and (for full games) unfair difficulty spikes or exploits.

## Step 4: Fix Bugs, Re-Verify, and Save Reusable Scripts

1. Fix every bug, visual glitch, ugly/broken texture or model, collision flaw, crosshair/weapon defect, or balance issue in code/assets, and re-run Step 2 & Step 3 with a fresh video + element close-ups until clean.
2. Save new or improved reusable scripts to `<gamedev_library>` sorted by **feature or physics topic first** when broadly reusable (`--scope feature --feature <name>`, `--scope physics --physics <topic>`), or by engine/element/game (`--scope engine`, `--scope element`, `--scope game`) with a clear `--summary` and `--tags` so future games can immediately recognize what each script tests. Never delete existing entries.
3. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
