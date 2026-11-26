---
name: prime-gamedev
description: Game design/engineering baseline (balance plan, ideal collisions, weapons & crosshair, game feel, HUD), dedicated multi-angle Asset Showcase scene visual QA for all custom and downloaded elements, collision-only live bot video recording, texture/model sourcing, and sorted script library for games and mods.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev Engineering & QA

Design and build games right from the start (concrete balance plan, airtight collisions with zero texture fall-throughs, mandatory crosshair and feedback whenever weapons exist, clean controls, fair enemies, readable HUD), then verify every game or mod using two focused, non-overlapping QA stages:
- **Stage A — Visual QA on a dedicated Showcase scene:** place every created AND downloaded visual element separately (characters, enemies, weapons, items, props, structures, effects, mod additions) in a standalone studio-lit showcase scene, run a fast script that photographs each element from multiple angles, inspect the photos with `view`, and fix/remake anything that looks wrong.
- **Stage B — Collision & physics QA via live bot video recording:** run a real live walking bot inside the gameplay scene under window/tab-scoped video recording strictly to verify collisions, physics, hitboxes, floor/texture fall-throughs, wall/corner clipping, stuck/softlock spots, runtime errors, and difficulty/balance — never teleporting around the map or using gameplay video to judge asset appearance.

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
2. **Strict separation of visual QA vs. collision video QA:**
   - **All visuals (custom AND downloaded models, sprites, textures, weapons, reticles, characters, props, structures, effects)** are verified on a dedicated standalone **Showcase scene** via fast multi-angle photo capture and `view` inspection. Never teleport around the gameplay map just to look at objects or judge asset design from gameplay video.
   - **Gameplay video recording + live bot** is used **strictly for collisions, physics, hitboxes, floor/texture fall-throughs, clipping, stuck/softlock spots, out-of-bounds, 3D camera wall clipping, controls/runtime errors, and difficulty/balance.**
3. **Never hijack the user's mouse or keyboard and never record the full computer screen.**
   - **Browser games & web tasks (when `browser_*` built-in browser tools are available):** do EVERYTHING completely inside Prime Desktop's built-in browser in the background (`browser_navigate`, `browser_record_start`, `browser_run`, `browser_press_key`, `browser_evaluate`, `browser_screenshot`, `browser_record_stop`). Do NOT open external Chrome/Playwright when built-in browser tools are available, and never move the user's physical cursor.
   - **Native/desktop games (or CLI-only web fallback without `browser_*`):** record strictly the target game window or isolated headless browser viewport (`native_bot_runner.mjs` / `web_bot_runner.mjs`), never the full computer desktop.
4. **Own Game vs. Mod Scope:**
   - **Full game you are building:** put all game elements on the Showcase scene for multi-angle visual inspection, and verify full-world collisions, gameplay loop, weapons/crosshair, UI/HUD, win/loss/restart states, and difficulty balance on the live bot run.
   - **Mod / plugin / addon for an existing third-party game:** put **only the mod's own elements** on the Showcase scene, and verify only the mod's own functionality and basic collisions — do NOT audit or try to fix unrelated bugs in the base game.
5. **Sourcing & polishing textures and 3D models:**
   - Unless the user asked you to craft textures/models yourself, first search online (`web_search`, `agentic_fetch`, `fetch`) and download ready, license-compatible assets via `download` (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt in `.png`, `.webp`, `.glb`, `.gltf`, `.obj`) into the project's asset folder.
   - Craft them yourself (via `generate_image`, procedural shaders/canvas scripts, or asset builders) whenever the user explicitly asks you to make them, or when no suitable asset exists online.
   - **Every downloaded and custom asset (including weapons) MUST be placed on the Showcase scene and photographed from multiple sides.** If any downloaded or custom asset looks ugly, cheap, mismatched in style/scale, or has UV/seam/geometry defects, fix or remake it yourself and re-shoot the Showcase.
6. **Reuse built-in presets & save context with `<gamedev_library>`.** Built-in reusable presets already ship under `<skill_directory>/scripts/presets/` (`showcase_capture_probe.mjs`, `teleport_collision_probe.mjs`, `weapon_crosshair_probe.mjs`, `physics_bounds_probe.mjs`, `balance_and_state_probe.mjs`) and are automatically indexed by `library.mjs lookup`. Always query `library.mjs lookup` first, read only the 1–2 matching scripts, and save new or improved reusable scripts into the proper sorted folder (`features/`, `physics/`, `engines/`, `elements/`, `games/`) without deleting existing ones.
7. **Fix every bug found and re-verify:** fix visual defects found on Showcase photos and re-shoot the Showcase; fix collision/physics/balance bugs found on the bot video/telemetry and re-run the collision bot video until clean.

## Step 1: Lookup Relevant Presets & Library Scripts (Context-Efficient)

```bash
node "<skill_directory>/scripts/library.mjs" lookup --feature "<feature>" --physics "<topic>" --engine "<engine>" --game "<game-slug>" --limit 6
```

Read **only** the 1–2 matching scripts returned by `lookup` (for example `presets/showcase_capture_probe.mjs`, `presets/teleport_collision_probe.mjs`, `presets/weapon_crosshair_probe.mjs`, or saved library entries). Never read unrelated folders.

## Step 2 (Stage A): Dedicated Asset Showcase Scene & Multi-Angle Photo Capture

1. Build a lightweight standalone Showcase mode/scene (for example `?showcase=1` in web games or a dedicated showcase scene/flag in native engines) with clean neutral lighting where **every standalone created and downloaded element** is isolated (player, enemies, weapons, reticles, items, props, structures, effects, mod additions).
2. Expose a lightweight inspection hook (`window.__SHOWCASE__.listElements()` and `window.__SHOWCASE__.focusElement(id, angle)` for web, or CLI/IPC cycling for native) supporting multiple sides (`front`, `right`, `back`, `left`, `quarter` for 3D; close-up + key animation/weapon states for 2D).
3. Run a fast automated photo pass (`presets/showcase_capture_probe.mjs`, or `browser_run` + `browser_screenshot` in the built-in browser, or native engine frame dump) to photograph every element from all sides quickly without navigating the gameplay map.
4. Inspect **every** generated showcase photo / contact sheet with `view` (when image viewing is supported) for ugly/broken models, weapon design flaws, mismatched style/scale, bad UVs/seams, missing textures, z-fighting, or broken shading. Fix or remake any flawed element and re-shoot until every asset looks clean.

## Step 3 (Stage B): Live Collision Bot & Window-Scoped Video Recording (Collisions & Physics Only)

### A. Browser Games & Web Apps in Prime Desktop (Preferred — Built-In Browser Only)

When built-in browser tools (`mcp_browser_*`) are available, run the entire collision test inside the isolated built-in browser tab without touching the user's mouse or launching external Playwright:
1. Open the gameplay URL in the built-in browser with `browser_navigate`.
2. Start background video recording of that tab with `browser_record_start` (`fps: 10`).
3. Drive a live collision-hunting bot inside the tab using `browser_run`, `browser_press_key` (`holdMs`), and `browser_evaluate` (entering gameplay, walking into walls/corners/seams/stairs/doors/edges, testing movement/combat/restart, optionally jumping to collision hotspots via `window.__GAME_DEBUG__.teleport` strictly for collision probing, and recording console/WebGL/asset errors, `NaN` physics, floor fall-throughs, FPS, and difficulty samples).
4. Stop recording with `browser_record_stop` to get `videoPath` (`playthrough.mp4`) and `framesDir`.
5. Slice `videoPath` (or `framesDir`) into dense contact sheets and `manifest.json` using `extract_frames.mjs`:

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

### C. Native / Desktop / Engine Games (Godot, Unity, Pygame, Bevy, SDL, Rust, C++, Mods)

Use `native_bot_runner.mjs` targeting **strictly the game window** (never the full computer desktop):

```bash
node "<skill_directory>/scripts/native_bot_runner.mjs" \
  --cmd "<launch command>" \
  --cwd "<game dir>" \
  --window "<Window Title>" \
  --bot-cmd "<bot driver command>" \
  --fps 4 \
  --max-frames 120
```

## Step 4: Collision Video Inspection (`view`), Fixes & Library Save

1. Read `report.json` (if any) and `frames/manifest.json`, then inspect the collision `contactSheets` (`sheet_*.png`) and flagged `frames/frame_XXXX.png` with `view` strictly for:
   - Falling through floors/textures, wall/geometry clipping, mismatched hitboxes, seam snags, out-of-bounds movement, 3D camera wall clipping, stuck/softlock spots
   - Missing/misaligned crosshair or broken weapon hit/reload behaviour (when weapons exist)
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, and (for full games) unfair difficulty spikes or broken restart state.
2. Fix every defect found and re-verify (Showcase photos for asset/design fixes; collision bot video for collision/physics fixes) until clean.
3. Save new or improved reusable scripts to `<gamedev_library>` sorted by **feature, element, or physics topic** (`--scope feature`, `--scope element`, `--scope physics`, `--scope engine`, `--scope game`) with clear `--summary` and `--tags`. Never delete existing entries.
4. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
