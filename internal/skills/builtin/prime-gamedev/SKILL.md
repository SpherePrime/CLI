---
name: prime-gamedev
description: Live playthrough bot testing, video-to-frames visual bug hunting, close-up element capture, collision/texture-clipping telemetry, difficulty verification, and sorted script library for games.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev QA

Verify every game by running a real, live playthrough bot that enters the game and hunts for bugs while recording a continuous video of the entire run, slicing the video into dense frames, capturing close-up screenshots of individual created elements, inspecting both via `view`, evaluating scripted collision/texture/error telemetry, checking difficulty balance, and fixing every bug found until a fresh video run passes cleanly.

When this file is loaded with `view`, Prime exposes `<skill_directory>` (ready-to-run base scripts under `scripts/`, updated on `prime update`) and `<gamedev_library>` (persistent user-data library of accumulated game/engine scripts outside the repo, never overwritten on updates).

## Core Rules

1. **Keep temporary videos and frames outside the repo** (the helper scripts default to `os.tmpdir()`) so `git status` stays clean.
2. **Never rely on unit tests or standalone screenshots alone.** Launch the real game, drive it with a live bot that actually enters gameplay through menus/start screens, and record a continuous video of the playthrough.
3. **Adapt to ANY game engine, language, or framework.** Inspect the project to see how the game launches, receives input, and renders, and wire live bot control + continuous video recording + frame slicing whether it is Web/Canvas/WebGL, Godot, Unity, Pygame, Bevy, SDL, C++/Rust, or terminal/CLI.
4. **Capture close-ups of individual created elements in addition to the playthrough video.** Shoot focused close-up screenshots of key elements you built or changed (player character, enemies, items, effects, HUD/UI panels, menus, win/loss screens) and inspect them with `view`.
5. **Save context with `<gamedev_library>`.** Never read all saved scripts blindly. Always run `library.mjs lookup` (or inspect `index.json`) first, read only the 1–2 relevant scripts matching the current game/engine, and after a successful run save new/improved reusable scripts into the proper folder without deleting existing ones.
6. **Fix every bug found and re-run.** Whenever frames, element close-ups, or telemetry reveal any bug, fix it in code and repeat the full live bot + video + frame/element inspection loop until clean.

## Step 1: Lookup Relevant Scripts (Context-Efficient)

Before writing a bot from scratch, query `<gamedev_library>/index.json` via `library.mjs` for scripts matching the current engine or game slug:

```bash
node "<skill_directory>/scripts/library.mjs" lookup --engine "<engine>" --game "<game-slug>" --limit 5
```

Read **only** the 1–2 matching scripts returned by `lookup` (for example `games/<game-slug>/...` or `engines/<engine>/...`), plus `<skill_directory>/scripts/` base helpers as needed. Never read unrelated game folders.

## Step 2: Run Live Bug-Hunting Bot + Video + Element Close-Ups

### Web / HTML5 / Canvas / WebGL / Three.js / Phaser Games

Start the dev server in the background and run `web_bot_runner.mjs`:

```bash
node "<skill_directory>/scripts/web_bot_runner.mjs" \
  --url "http://127.0.0.1:5173" \
  --bot "./path/to/bot.mjs" \
  --fps 4 \
  --max-frames 120
```

Inside `--bot` (`export async function runBot({ page, canvas, helpers })`), make the bot enter the real game, complete a full loop, stress-test corners/walls/floor seams/boundaries/rapid inputs, and capture close-ups of created elements:
- `helpers.holdKey(key, durationMs)`, `helpers.tapKey(key, holdMs)`
- `helpers.clickCanvas(relX, relY)`, `helpers.moveMouseCanvas(relX, relY)`
- `helpers.captureElement(selector, label, { padding })` — close-up of a DOM/HUD/menu/UI element
- `helpers.captureRegion({ x, y, width, height }, label)` — close-up of a specific character/enemy/object area on canvas
- `helpers.captureEventFrame(label)` — full screen at a key state (title, gameplay, boss, win, loss)
- `helpers.markCheckpoint(name, meta)`, `helpers.readGameState()`, `helpers.recordDifficultySample(sample)`

You can also capture standalone element close-ups anytime with `element_capture.mjs`:

```bash
node "<skill_directory>/scripts/element_capture.mjs" \
  --url "http://127.0.0.1:5173" \
  --selectors "hud=.hud;menu=.start-screen" \
  --crops "player:520,280,220,220" \
  --zoom 2
```

### Native / Desktop / Engine Games (Godot, Unity, Pygame, Bevy, SDL, Rust, C++, etc.)

Use `native_bot_runner.mjs` (or launch the game with your bot input driver and record continuously via `ffmpeg`):

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

Or slice any recorded video (`.mp4`/`.webm`/`.mkv`) or engine-dumped PNG directory with `extract_frames.mjs`, and crop element close-ups from a representative frame with `element_capture.mjs --image <frame.png> --crops "..."`.

## Step 3: Visual Inspection (`view`) + Telemetry + Difficulty

1. Read `report.json` and `frames/manifest.json`.
2. Inspect **all** generated `contactSheets` (`sheet_*.png`) with `view` chronologically, then inspect individual `frames/frame_XXXX.png` for any anomalies, transitions, or suspected bugs.
3. Inspect **all** `elementCaptures` / `elements/*.png` close-ups with `view` to verify each created element (character, enemies, items, HUD, menus, visual polish) looks crisp and defect-free.
4. Hunt specifically for:
   - Falling through floors/textures, wall/geometry clipping, out-of-bounds movement
   - Missing, checkerboard, stretched, or flickering (z-fighting) textures
   - Broken shaders/lighting, black/frozen frames, particle leaks, HUD overlap/clipping, broken animations
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, softlocks, and unfair difficulty spikes or exploits.

## Step 4: Fix Bugs, Re-Verify, and Save Reusable Scripts

1. If ANY bug, visual glitch, texture issue, or balance flaw appears, fix it in the game code and re-run Step 2 & Step 3 with a fresh video + element close-ups until clean.
2. Save new or improved bot / element-capture scripts to `<gamedev_library>` sorted by folder (`--scope game --game <slug>`, `--scope engine --engine <name>`, or `--scope element --category <pattern>`). Never delete existing entries:

```bash
node "<skill_directory>/scripts/library.mjs" save \
  --scope game \
  --game "<project-slug>" \
  --engine "<engine>" \
  --file "/tmp/my_bot.mjs" \
  --name "bot.mjs" \
  --tags "platformer,collision,hud" \
  --summary "Full-loop bug-hunting bot and element captures for <project-slug>"
```

3. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
