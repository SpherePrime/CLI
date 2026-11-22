---
name: prime-gamedev
description: Live playthrough bot testing, video-to-frames visual bug hunting, collision/texture-clipping telemetry, and difficulty balance verification for games.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev QA

Verify every game by running a real, live playthrough bot that actively hunts for bugs while recording the entire run, slicing the video into a dense frame sequence, inspecting those frames visually with `view`, evaluating scripted collision/texture/error telemetry, and checking difficulty balance.

When this file is loaded with `view`, Prime exposes `<skill_directory>` containing ready-to-run helper scripts under `scripts/`.

## Core Rules

1. **Always store temporary artifacts outside the repository** (use system temp, which the helper scripts default to) so project files and `git status` stay clean.
2. **Never rely on unit tests alone.** Launch the real game (browser/Canvas/WebGL or native executable) and drive it with a live bot.
3. **Hunt for bugs actively, do not only walk a happy path.** Probe walls, corners, floor seams, ledges, rapid inputs, pause/restart cycles, boundary limits, and worst-case physics collisions.
4. **Record and slice the full run into dense frames**, then **visually inspect contact sheets and flagged/key frames with `view`** whenever your model supports images.

## Workflow

### 1. Prepare a Bug-Hunting Playthrough Bot

Write a bot tailored to the game that:
- Completes at least one full game loop / level start-to-finish (plus win, loss, and restart states).
- Actively stress-tests physics and geometry:
  - Walks/dashes/jumps directly into corners, walls, seams, moving platforms, and map edges to provoke falling through floors/textures or wall clipping.
  - Tests out-of-bounds movement, camera limits, rapid simultaneous key presses, and state transitions (pause/resume/restart/respawn).
- Exposes or reads game telemetry when possible (for example `window.__GAME_DEBUG__` in web games or a debug log/JSON telemetry sink in native games): player coordinates `(x, y, z)`, velocity, grounded state, current level, HP, score, enemy counts, FPS, and entity counts.

### 2. Run the Bot with Video Recording and Scripted Checks

#### Web / HTML5 / Canvas / WebGL / Three.js / Phaser Games

Start the game server in the background (or use an existing URL) and run `web_bot_runner.mjs`:

```bash
node "<skill_directory>/scripts/web_bot_runner.mjs" \
  --url "http://127.0.0.1:5173" \
  --bot "./path/to/bot.mjs" \
  --fps 4 \
  --max-frames 120
```

What `web_bot_runner.mjs` does automatically:
- Launches Chromium (with SwiftShader WebGL enabled so 3D/Canvas games render cleanly headless).
- Records full-viewport video of the entire bot playthrough into a temp directory.
- Captures fallback periodic PNG screenshots if `ffmpeg` is unavailable.
- Tracks runtime failures: uncaught `pageerror`, `console.error`, WebGL/shader warnings, failed requests and HTTP `>= 400` asset/texture loads, and blank/frozen `<canvas>` checks.
- Polls game state (via `window.__GAME_DEBUG__` or custom `--state-expr`) to catch:
  - Falling through floors/textures or out-of-bounds coordinates
  - `NaN` / `Infinity` in physics/coordinates/HP/score
  - Stuck/softlock states (input held while position does not change)
  - FPS drops and difficulty metrics (deaths, completion time, win/loss ratio, progression pacing)
- Automatically invokes `extract_frames.mjs` to slice the recorded video into dense frames and contact sheets, writing `report.json` and `frames/manifest.json`.

A bot module passed to `--bot` must export `runBot({ page, canvas, helpers })`:
- `helpers.holdKey(key, durationMs)`
- `helpers.tapKey(key, holdMs)`
- `helpers.clickCanvas(relX, relY)`
- `helpers.moveMouseCanvas(relX, relY)`
- `helpers.markCheckpoint(name, meta)`
- `helpers.captureEventFrame(label)`
- `helpers.readGameState()`
- `helpers.recordDifficultySample(sample)`

If no `--bot` file is passed, `web_bot_runner.mjs` runs a built-in exploratory/collision-probing bot across canvas regions and movement/action keys (`Arrow*`, `WASD`, `Space`, `Enter`, clicks).

#### Native / Desktop / Engine Games (Godot, Unity, Pygame, Bevy, SDL, Love2D, etc.)

1. Launch the game with your automated playthrough/input-driving bot and record gameplay video into a temp path:
   - Windows: `ffmpeg -y -f gdigrab -framerate 30 -i title="<WindowTitle>" -t 45 "$TEMP/playthrough.mp4"` (or desktop capture / engine movie recorder)
   - Linux: `ffmpeg -y -f x11grab -framerate 30 -i :0.0 -t 45 "/tmp/playthrough.mp4"`
   - macOS: `ffmpeg -y -f avfoundation -framerate 30 -i "1:none" -t 45 "/tmp/playthrough.mp4"`
   - Or have the game/test harness save sequential PNG frames directly into a temp directory.
2. Slice the video (or pack an existing frame directory) into contact sheets and `manifest.json`:

```bash
node "<skill_directory>/scripts/extract_frames.mjs" \
  --video "/tmp/playthrough.mp4" \
  --fps 4 \
  --max-frames 120
```

Or from engine-dumped PNGs:

```bash
node "<skill_directory>/scripts/extract_frames.mjs" \
  --frames-dir "/tmp/raw-frames" \
  --max-frames 120
```

### 3. Visually Inspect Extracted Frames with `view`

Read `report.json` (for web) and `frames/manifest.json` first, then visually inspect the generated images with `view`:
1. Open **all** `contactSheets` (`sheet_001.png`, `sheet_002.png`, ...) with `view` to scan the entire playthrough chronologically.
2. Open individual full-resolution `frames/frame_XXXX.png` (and any `events/*.png` or `anomalies` flagged in `manifest.json`) with `view` wherever anything looks suspicious, blurry, clipped, black, frozen, or during critical gameplay transitions.
3. Specifically hunt for visual bugs:
   - **Geometry & texture clipping**: player/NPCs/enemies sinking into or falling through floors, walls, or terrain; camera clipping inside geometry.
   - **Texture & asset defects**: missing/pink/checkerboard/corrupt textures, stretched UVs, flickering/z-fighting surfaces, misaligned hitboxes vs visuals.
   - **Render & shader glitches**: black/blank frames, broken lighting, particle leaks, stuck overlays.
   - **UI & HUD defects**: overlapping text, clipped buttons/bars, off-screen prompts, unreadable contrast during action.
   - **Animation & state bugs**: frozen walk/idle poses, T-poses, desynced death/respawn visuals.

### 4. Verify Difficulty, Balance, and Softlocks

Check bot telemetry and frame timings to verify:
- The game loop is completable end-to-end without softlocks or unreachable objectives.
- Difficulty pacing is fair: no unavoidable instant-kill spikes, impossible jumps/timers, or trivial exploits that bypass challenge.
- Win/loss conditions, retries, score/resource curves, and enemy/obstacle speeds match intended difficulty.

### 5. Cleanup

When finished, remove temporary artifact directories created in `os.tmpdir()` unless the user asked to keep recordings.
