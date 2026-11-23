---
name: prime-gamedev
description: Live playthrough bot testing, built-in browser / window-scoped video recording, video-to-frames visual bug hunting, close-up element capture, collision/texture-clipping telemetry, texture/model sourcing, difficulty verification, and sorted script library for games and mods.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev QA

Verify every game or mod by running a real, live playthrough bot that enters the game, walks and moves through gameplay thoroughly, records continuous video without taking over the user's mouse or keyboard, slices the video into dense frames, captures close-up screenshots of individual created elements, inspects both via `view`, evaluates scripted collision/texture/error telemetry, checks difficulty balance, and fixes every bug found until a fresh video run passes cleanly.

When this file is loaded with `view`, Prime exposes `<skill_directory>` (ready-to-run base scripts under `scripts/`, updated on `prime update`) and `<gamedev_library>` (persistent user-data library of accumulated game/engine scripts outside the repo, never overwritten on updates).

## Core Rules

1. **Keep temporary videos and frames outside the repo** (the helper scripts and `browser_record_start` default to `os.tmpdir()`) so `git status` stays clean.
2. **Never rely on unit tests or standalone screenshots alone.** Launch the real game, drive it with a live bot that actually enters gameplay through menus/start screens, and record a continuous video of the playthrough.
3. **Never hijack the user's mouse or keyboard and never record the full computer screen.**
   - **Browser games & web tasks (when `browser_*` built-in browser tools are available):** do EVERYTHING completely inside Prime Desktop's built-in browser in the background (`browser_navigate`, `browser_record_start`, `browser_run`, `browser_press_key`, `browser_evaluate`, `browser_screenshot`, `browser_record_stop`). Do NOT open external Chrome/Playwright when the built-in browser tools are available, and never move the user's physical cursor.
   - **Native/desktop games (or CLI-only web fallback without `browser_*`):** record strictly the target game window or isolated headless browser viewport (`native_bot_runner.mjs` / `web_bot_runner.mjs`), never the full computer desktop.
4. **Own Game vs. Mod Scope:**
   - **Full game you are building:** verify the whole game end-to-end — walk through all zones/levels, probe walls, corners, floor seams, edges, enemies, items, triggers, textures, 3D models, collisions, UI/HUD, win/loss/restart states, and difficulty balance, fixing every bug found.
   - **Mod / plugin / addon for an existing third-party game:** verify **specifically the mod's own functionality** — check how the mod's added/modified textures, models, UI, mechanics, and basic collisions look and behave in-game. Do NOT spend time auditing or trying to fix bugs in the unrelated base game that you did not build.
5. **Walk and probe thoroughly inside the game (applies to every engine & platform).** Do not only click "Play" and stand still. The bot must actively walk/move across the playable space (all directions, jumps, dashes, stairs, doors, room transitions), press against walls, corners, floor/terrain seams, ledges, and map boundaries, trigger interactions/combat/items, test HUD/pause/restart, and probe for falling through floors/textures, wall clipping, missing/stretched/flickering textures (z-fighting), `NaN` physics, softlocks, and console/engine errors.
6. **Capture close-ups of individual created elements in addition to the playthrough video.** Shoot focused close-up screenshots of key elements you built or changed (player character, enemies, items, effects, HUD/UI panels, menus, mod items/blocks) and inspect them with `view`.
7. **Sourcing & polishing textures and 3D models:**
   - Unless the user asked you to craft textures/models yourself, first search online (`web_search`, `agentic_fetch`, `fetch`) and download ready, license-compatible assets via `download` (e.g. Poly Haven, Kenney, Poly Pizza, AmbientCG, OpenGameArt in `.png`, `.webp`, `.glb`, `.gltf`, `.obj`) into the project's asset folder.
   - Craft them yourself (via `generate_image`, procedural shaders/canvas scripts, or asset builders) whenever the user explicitly asks you to make them, or when no suitable asset exists online.
   - Always inspect integrated assets in-game via `view` (video frames and element close-ups). If any downloaded or custom texture/model looks ugly, cheap, mismatched in style/scale, or has UV/seam/rendering defects, fix or remake it yourself.
8. **Save context with `<gamedev_library>`.** Never read all saved scripts blindly. Always run `library.mjs lookup` (or inspect `index.json`) first, read only the 1–2 relevant scripts matching the current game/engine, and after a successful run save new/improved reusable scripts into the proper folder without deleting existing ones.
9. **Fix every bug found and re-run.** Whenever frames, element close-ups, or telemetry reveal any bug, fix it in code/assets and repeat the full live bot + video + frame/element inspection loop until clean.

## Step 1: Lookup Relevant Scripts (Context-Efficient)

Before writing a bot from scratch, query `<gamedev_library>/index.json` via `library.mjs` for scripts matching the current engine or game slug:

```bash
node "<skill_directory>/scripts/library.mjs" lookup --engine "<engine>" --game "<game-slug>" --limit 5
```

Read **only** the 1–2 matching scripts returned by `lookup`, plus `<skill_directory>/scripts/` base helpers as needed. Never read unrelated game folders.

## Step 2: Run Live Walking Bot + Video + Element Close-Ups

### A. Browser Games & Web Apps in Prime Desktop (Preferred — Built-In Browser Only)

When built-in browser tools (`mcp_browser_*`) are available, run the entire test inside the isolated built-in browser tab without touching the user's mouse or launching external Playwright:
1. Start the game dev server in the background (if needed) and open the game URL in the built-in browser with `browser_navigate` (using this chat's tab).
2. Start background video recording of that tab with `browser_record_start` (`fps: 10`).
3. Drive a real walking & bug-hunting bot inside the tab using:
   - `browser_run` with `click{selector|text|x,y}` (to enter Start/Play menus or canvas buttons), `press_key{key,holdMs}` (real held movement keys like `ArrowRight`/`KeyD`/`KeyW`/`Space` for 300–1200ms so the player actually walks across levels, corners, walls, and seams), and `evaluate{expression}` (installing console/WebGL/asset-error hooks and reading telemetry such as player coordinates, grounded state, collisions, floor-fall-throughs, `NaN`, stuck/softlock checks, FPS, and difficulty samples).
   - `browser_press_key` (`holdMs`) and `browser_evaluate` as needed for multi-stage exploration across zones/levels.
   - `browser_screenshot` (`clip`, `ref`, `zoom: 2`) for close-up captures of created characters, textures, models, and HUD/UI elements.
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

Inside `--bot` (`export async function runBot({ page, canvas, helpers })`), use `helpers.holdKey`, `helpers.tapKey`, `helpers.clickCanvas`, `helpers.captureElement`, `helpers.captureRegion`, `helpers.captureEventFrame`, `helpers.readGameState`, `helpers.recordDifficultySample`.

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

Slice any recorded video or engine-dumped PNG frames with `extract_frames.mjs`, and crop element close-ups with `element_capture.mjs --image <frame.png> --crops "..."`.

## Step 3: Visual Inspection (`view`) + Telemetry + Difficulty

1. Read `report.json` (if any) and `frames/manifest.json`.
2. Inspect **all** generated `contactSheets` (`sheet_*.png`) with `view` chronologically, then inspect individual `frames/frame_XXXX.png` for anomalies, transitions, or suspected bugs.
3. Inspect **all** `elementCaptures` / `elements/*.png` / `browser_screenshot` close-ups with `view` to verify each created element, texture, model, and HUD component looks crisp and defect-free.
4. Hunt specifically for:
   - Falling through floors/textures, wall/geometry clipping, out-of-bounds movement, stuck/softlock spots
   - Missing, checkerboard, stretched, seam-broken, or flickering (z-fighting) textures and ugly/broken 3D models
   - Broken shaders/lighting, black/frozen frames, particle leaks, HUD overlap/clipping, broken animations
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, and (for full games) unfair difficulty spikes or exploits.

## Step 4: Fix Bugs, Re-Verify, and Save Reusable Scripts

1. Fix every bug, visual glitch, ugly/broken texture or model, collision flaw, or balance issue in code/assets, and re-run Step 2 & Step 3 with a fresh video + element close-ups until clean.
2. Save new or improved bot / element-capture scripts to `<gamedev_library>` sorted by folder (`--scope game --game <slug>`, `--scope engine --engine <name>`, or `--scope element --category <pattern>`). Never delete existing entries.
3. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
