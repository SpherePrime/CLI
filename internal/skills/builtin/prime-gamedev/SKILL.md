---
name: prime-gamedev
description: Game design/engineering baseline (balance plan, ideal collisions, weapons & crosshair, game feel, HUD), dedicated multi-angle Asset Showcase scene visual QA for all custom and downloaded elements, deterministic code-level collision testing across all sides, live gameplay bot video recording, texture/model sourcing, and sorted script library for games and mods.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev Engineering & QA

Design and build games right from the start (concrete balance plan, airtight collisions with zero texture fall-throughs, mandatory crosshair and feedback whenever weapons exist, clean controls, fair enemies, readable HUD), then verify every game or mod using three focused, non-overlapping QA stages:
- **Stage A — Visual QA on a dedicated Showcase scene + Assembled Map/Level sweep:** place every created, engine-library, AND downloaded visual element separately (characters, enemies, weapons, items, props, structures, effects, mod additions) in a standalone studio-lit Showcase scene and photograph each from multiple angles; ALSO capture the **assembled map/level** (overhead/flyover + key gameplay-camera views) and inspect everything with `view` for floating/sunk props, wall/floor seams, broken scale, lighting holes, harsh tiling, or style mismatch.
- **Stage B — Deterministic collision & physics tests in code (NO in-game teleporting):** write and run real automated code tests verifying that every solid object, wall, floor, prop, door, and item actually has a collider matching its rendered visual bounds, blocks movement from all sides (all 4 sides in 2D / all 6 faces in 3D), handles corners and seams cleanly, prevents high-speed tunneling, and enforces map bounds and 3D camera collision.
- **Stage C — Live gameplay bot video check:** run a real live gameplay bot inside the game under window/tab-scoped video recording to verify controls, menu/pause/restart flow, combat/crosshair, runtime errors, softlocks, FPS, and difficulty/balance — without teleporting around the map and without using gameplay video to judge individual asset appearance.

When this file is loaded with `view`, Prime exposes `<skill_directory>` (ready-to-run base scripts under `scripts/`, updated on `prime update`) and `<gamedev_library>` (persistent user-data library of accumulated game/engine scripts outside the repo, never overwritten on updates).

## Game Design & Engineering Baseline (Build Right From the Start)

Read `prime://skills/prime-gamedev/references/game-craft.md` for the full **Universal Game Quality Standard**, **Scale to Scope** (mini-game vs. standalone medium/large full game vs. third-party mod), and **Genre-Specific Standards** (Shooters, Melee/RPG, Platformers, Racing, Strategy/TD, Horror/Survival, Puzzles/Card — apply ONLY the genre rules that fit the current game).

Follow these core fundamentals proactively while designing and coding (for a third-party game mod, apply them strictly to the mod's own elements, weapons, structures, collisions, UI, and balance):

1. **Balance Plan & State Architecture Before Coding:**
   - Draft a concrete balance plan before writing gameplay code: movement speed, jump height/reach, player and enemy HP, damage, time-to-kill (TTK), cooldowns, economy/pickups, difficulty curve across levels/waves, and win/loss conditions.
   - Implement a clean state lifecycle (`Menu -> Gameplay -> Pause -> Win/Loss -> Restart`) where Restart completely resets timers, enemies, projectiles, physics velocity/positions, and score.
   - Use a fixed physics timestep (or clamp `dt <= 0.05s`) so frame spikes never tunnel entities through floors or walls.
2. **Ideal Collisions & Zero Texture Fall-Throughs:**
   - Maintain a single source of truth between visuals and hitboxes: colliders must accurately match rendered sprites/meshes (no invisible walls, hollow solid-looking meshes, or offset hitboxes).
   - Resolve collisions cleanly per axis (X/Y/Z) and use continuous/swept checks for fast actors and projectiles to prevent floor fall-throughs, corner snagging, and wall tunneling.
   - Implement reliable ground checks, clean handling of tile/platform seams, stairs, ceilings, and hard map boundaries (plus out-of-bounds kill-zone/recovery).
   - In 3D / third-person views, add camera collision so the camera never clips inside walls or geometry.
   - Keep collision world definitions and step resolution decoupled from rendering so automated code tests can run directly against the real collision engine.
3. **Weapons, Mandatory Crosshair, True ADS & Combat (Whenever Combat Exists):**
   - **Crosshair is mandatory:** if the game includes any gun, bow, throwable, turret, or aimed spell, include a crisp, high-contrast crosshair/reticle (screen center for 3D/FPS, cursor-aligned for 2D/top-down) with visual feedback for firing/spread, hitmarker on hit, and reload state.
   - **True Aim Down Sights (ADS) — NEVER just fake camera zoom:** whenever a first-person/3D weapon has sights, optics, or ADS aiming, smoothly move the weapon model to center and align the camera line of sight directly through the weapon's actual iron sights, red-dot, or scope (with synchronized sight alignment, subtle FOV shift, and reduced spread) — do NOT just zoom the camera FOV while leaving the weapon hanging off to the side.
   - Align the shot/ray origin with the crosshair/sights and weapon muzzle; provide visible tracers/projectiles, muzzle flash, recoil/kick, cooldown, ammo/reload loop, and fair hitboxes.
   - For melee combat, provide a readable swing arc/telegraph, knockback, and brief invulnerability frames (i-frames) after taking damage so the player is never deleted in a single frame.
4. **Controls & Game Feel:**
   - Normalize diagonal movement so diagonal speed is not faster; keep input snappy; add coyote time and jump buffering for platformers.
   - Give tangible feedback for actions: hit flashes, particles, subtle screen shake, animations, and clear pickup/damage/death cues.
5. **Complete, Animated, Armed Enemies, Real Death & Smart AI (Genre-Appropriate):**
   - **Never build bare, empty-handed enemies that deal invisible damage.** Match enemies to the game's genre and plan them upfront:
     - Equip combat enemies with **visible weapons/gear properly attached to their hand sockets** (guns, rifles, swords, bows, staves, claws) — verify enemies holding their weapons on the Showcase scene too.
     - Give enemies **real animations** (idle, walk/run, aim/wind-up, fire/swing, hit reaction, death).
     - Give enemies **visible, physically grounded attacks** — in shooters, enemies MUST actually aim and fire visible projectiles/tracers with muzzle flashes from their weapon barrel; in melee/action/RPG games, enemies MUST telegraph and execute visible weapon swings, spells, or lunge attacks so the player can read and dodge/take cover.
   - **Real hit reactions & death (NEVER vanish in 1 frame):** show impact feedback at the hit point; on `hp <= 0`, play a real death animation / fall / ragdoll / shatter, immediately disable collision & damage so corpses never block or hurt the player, and fade/pool cleanly within a cap.
   - Give AI real obstacle avoidance / pathing, line-of-sight checks (never shooting through solid walls), separation (never stacking onto identical coordinates), and validated clear spawns (never inside walls or right behind the player); guarantee zero softlocks.
6. **Coherent Art Direction, Atmosphere, Input Hygiene, Audio & Pooling:**
   - Pick ONE art direction and palette upfront and unify downloaded/custom assets to match; preload assets cleanly before level start; build atmospheric worlds (landmarks, lighting, skybox/fog, subtle ambient life) rather than empty gray boxes.
   - Enforce camera & input hygiene: clamp 3D pitch `±85°`, spring-arm camera collision, auto-pause + clear held keys on `blur`/`visibilitychange`, FPS-independent mouse look, clean `Pointer Lock` + `resize`, pleasant non-harsh audio with volume/Mute, persistent settings/scores, object pooling, and leak-free `Restart` (cancel old loops/timers so speed never doubles after restarts).
   - Keep HUD clean and readable on any background (subtle backing/shadow, clear objective, styled screens — never bare browser `alert()`).
7. **Performance Budget (`>= 50 FPS`) & Visual/Graphics Settings (Plan Upfront):**
   - Design for smooth performance: target **at least 50 FPS** during active gameplay (use instancing/batching, object pooling for projectiles/particles, frustum culling/LOD, bounded shadow-casting lights, and appropriately sized textures).
   - For any **standalone medium or large full game** (especially 3D or visually rich projects), plan and implement **real working Graphics / Quality Settings** accessible from Main Menu and Pause Menu: overall quality preset (`Low / Medium / High`), shadow quality (`Off / Low / Medium / High`), draw distance / LOD / effects & particle density, post-processing / antialiasing, and render scale / DPR cap (plus audio & mouse sensitivity where applicable). Every setting MUST actually change the renderer/performance — never build decorative non-functional toggles. **Do NOT build a graphics settings menu for a mod/plugin/addon to an existing third-party game** (the host game already owns graphics settings — for mods, simply keep the mod's own features from dropping FPS below 50).

## Core QA Rules

1. **Keep temporary videos and frames outside the repo** (the helper scripts and `browser_record_start` default to `os.tmpdir()`) so `git status` stays clean.
2. **Strict three-way separation (no teleport hacks):**
   - **Stage A (Showcase + Assembled Map visuals):** all isolated visuals (custom, engine-library, AND downloaded models, sprites, textures, weapons/ADS, reticles, armed enemies, props, structures, effects) are verified on a dedicated standalone **Showcase scene** via fast multi-angle photo capture, AND for full games/levels the **assembled map/level** is screenshotted (overhead/flyover + gameplay-camera views) and inspected with `view` for floating/sunk props, wall/floor seams, broken scale, lighting holes, and harsh tiling.
   - **Stage B (Code collision tests):** all object colliders, visual-vs-hitbox alignment, all-sides blocking (4 sides in 2D / 6 faces in 3D), corners, seams, tunneling, bounds, and 3D camera walls are verified by **deterministic automated tests in code** — NEVER by teleporting a bot around the gameplay map.
   - **Stage C (Live gameplay video):** a live bot in the built-in browser / target game window verifies real controls, gameplay loop, combat/crosshair, menu/restart, runtime errors, softlocks, and difficulty/balance.
3. **Never hijack the user's mouse or keyboard and never record the full computer screen.**
   - **Browser games & web tasks (when `browser_*` built-in browser tools are available):** do EVERYTHING completely inside Prime Desktop's built-in browser in the background (`browser_navigate`, `browser_record_start`, `browser_run`, `browser_press_key`, `browser_evaluate`, `browser_screenshot`, `browser_record_stop`). Do NOT open external Chrome/Playwright when built-in browser tools are available, and never move the user's physical cursor.
   - **Native/desktop games (or CLI-only web fallback without `browser_*`):** record strictly the target game window or isolated headless browser viewport (`native_bot_runner.mjs` / `web_bot_runner.mjs`), never the full computer desktop.
4. **Own Game vs. Mod Scope:**
   - **Full game you are building:** put all game elements on the Showcase scene for multi-angle visual inspection, cover all world objects in code collision tests, and verify gameplay loop, weapons/crosshair, UI/HUD, win/loss/restart states, and difficulty balance on the live bot run.
   - **Mod / plugin / addon for an existing third-party game:** put **only the mod's own elements** on the Showcase scene, and test only the mod's own colliders and functionality — do NOT audit or try to fix unrelated bugs in the base game.
5. **Sourcing, adapting & polishing assets (DEFAULT = DOWNLOAD & ADAPT — NEVER RELY ON ONE SITE):**
   - Unless the user explicitly asks you to craft models, weapons, textures, or animations from scratch, **by default download and adapt ready assets**:
     1. Check **native engine & project libraries first** (Unreal Engine: `Content/`, Vault, `Starter Content` / `Engine Content`, **Fab / Quixel Megascans**, **MetaHuman**, **Game Animation Sample**; Unity: Asset Store Free; Godot: AssetLib).
     2. Read `prime://skills/prime-gamedev/references/asset-sources.md` with `view` when sourcing web assets — it lists curated sources by style & category (realistic 3D & photogrammetry, 3D weapons, character animations & auto-rigging, stylized 3D, realistic & stylized PBR textures, 2D/HUD/HDRI/audio) clearly split into **`[Direct / API / No-Login]`** (first priority for automated `download`, never waste turns scraping login walls!) vs **`[Account / Engine Plugin / Manual]`**. You may also search the web freely when needed.
     3. Craft from scratch **only** when the user explicitly requests it or no suitable asset exists.
   - **Always adapt & fix downloaded models, weapons, textures, and animations:** normalize metre scale, weapon hand-grip/muzzle pivot, PBR maps, and skeleton retargeting (foot-ground contact, root motion, limb twisting, blends/timing). **If any downloaded asset or animation looks bad or even slightly awkward, edit/fix or replace it yourself** and verify every element on the Showcase scene.
6. **Reuse built-in presets & save context with `<gamedev_library>`.** Built-in reusable presets already ship under `<skill_directory>/scripts/presets/` (`showcase_capture_probe.mjs`, `collision_suite_probe.mjs`, `weapon_crosshair_probe.mjs`, `physics_bounds_probe.mjs`, `balance_and_state_probe.mjs`) and are automatically indexed by `library.mjs lookup`. Always query `library.mjs lookup` first, read only the 1–2 matching scripts, and save new or improved reusable scripts into the proper sorted folder (`features/`, `physics/`, `engines/`, `elements/`, `games/`) without deleting existing ones.
7. **Bot liveness, hard timeouts & anti-hang (MANDATORY):**
   - Never leave a live bot or game running blindly in the background. Always run bot scripts with a hard timeout (`--bot-timeout-ms`, bounded step loops) and clean teardown so a broken script can never hang an app open forever.
   - Immediately verify the bot is actually driving the app: inspect bot stdout/stderr AND confirm visible progress (menu -> gameplay transition, player position/state delta, or non-identical frames).
   - If the screen stays static/frozen (`static_screen` / `mostly_static_screen` / `identical_frame_sizes`), inputs have no effect, focus was lost, or the bot times out, **stop/kill the stuck process immediately, diagnose and fix the bot script or game input handling, and rerun** — NEVER treat an unresponsive/static run as a pass.
8. **Fix every bug found and re-verify:** fix visual defects found on Showcase photos (re-shoot Showcase), fix collision flaws found by code collision tests (re-run test suite), and fix gameplay/runtime/balance bugs (re-run live bot video) until clean.

## Step 1: Lookup Relevant Presets & Library Scripts (Context-Efficient)

```bash
node "<skill_directory>/scripts/library.mjs" lookup --feature "<feature>" --physics "<topic>" --engine "<engine>" --game "<game-slug>" --limit 6
```

Read **only** the 1–2 matching scripts returned by `lookup` (for example `presets/showcase_capture_probe.mjs`, `presets/collision_suite_probe.mjs`, `presets/weapon_crosshair_probe.mjs`, or saved library entries). Never read unrelated folders.

## Step 2 (Stage A): Dedicated Asset Showcase Scene + Assembled Map/Level Visual Sweep

1. Build a lightweight standalone Showcase mode/scene (for example `?showcase=1` in web games or a dedicated showcase scene/flag in native engines) with clean neutral lighting where **every standalone created, engine-library, and downloaded element** is isolated (player, enemies with weapons in hand, weapons & ADS sight view, reticles, items, props, structures, effects, mod additions).
2. Expose a lightweight inspection hook (`window.__SHOWCASE__.listElements()` and `window.__SHOWCASE__.focusElement(id, angle)` for web, or CLI/IPC cycling for native) supporting multiple sides (`front`, `right`, `back`, `left`, `quarter` for 3D; close-up + key animation/weapon states for 2D).
3. Run a fast automated photo pass (`presets/showcase_capture_probe.mjs`, or `browser_run` + `browser_screenshot` in the built-in browser, or native engine frame dump) to photograph every element from all sides quickly.
4. **Assembled Map / Level Sweep (for full games/levels):** capture screenshots of the **assembled map/level** from overhead/flyover angles and gameplay-camera viewpoints at key areas (skip for minimal single-screen 2D puzzles where the main screen already shows the whole board).
5. Inspect **every** generated Showcase photo, contact sheet, and assembled-map screenshot with `view` (when image viewing is supported) for:
   - ugly/broken models, weapon/ADS flaws, crooked enemy weapon grips or animations, mismatched art style/scale, bad UVs/seams, missing textures, z-fighting;
   - floating or sunk props, gaps between walls/floors/corners, pitch-black corners or blown-out lighting, harsh repeating tiling.
   Fix or remake any flawed element or level placement and re-shoot until clean.

## Step 3 (Stage B): Deterministic Automated Collision Tests in Code (NO Teleporting)

Write and run real automated collision & physics tests directly against the game's collision world and physics step function (using Vitest/Node, `cargo test`, `go test`, `pytest`, Godot/Unity test runner, or `presets/collision_suite_probe.mjs`):
1. **Collider presence & visual alignment:** iterate over every solid object, wall, floor, platform, prop, door, and collidable item in the scene/level definition and assert each has a real collider whose bounds match its rendered sprite/mesh within tolerance (zero hollow visual solids, zero phantom invisible walls).
2. **All-sides blocking:** for every solid collider, drive a test actor/hitbox into **every side** (`+X`, `-X`, `+Y`, `-Y` in 2D; plus `+Z`, `-Z` in 3D) and assert zero penetration and finite coordinates (`!NaN`, `!Infinity`).
3. **Corners, seams, stairs, and high-speed tunneling:** test diagonal movement into corners, walking across adjacent floor/platform seams, stepping with large `dt` spikes (verifying `dt <= 0.05s` clamp / swept checks prevent tunneling through thin floors or walls), hard map bounds / out-of-bounds recovery, projectile/weapon hitboxes, and 3D camera wall collision.
4. Fix every failing collision case in code and re-run the test suite until 100% green.

## Step 4 (Stage C): Live Gameplay Bot & Window-Scoped Video Check

### A. Browser Games & Web Apps in Prime Desktop (Preferred — Built-In Browser Only)

When built-in browser tools (`mcp_browser_*`) are available, run the live gameplay test inside the isolated built-in browser tab without touching the user's mouse or launching external Playwright:
1. Open the gameplay URL in the built-in browser with `browser_navigate`.
2. Start background video recording of that tab with `browser_record_start` (`fps: 10`).
3. Drive a live gameplay bot inside the tab using `browser_run`, `browser_press_key` (`holdMs`), and `browser_evaluate` (entering gameplay through menus, testing real controls/movement, aiming/firing with crosshair when weapons exist, pause/restart reset, and recording console/WebGL/asset errors, `NaN` physics, FPS, and difficulty samples — without teleporting around the map).
4. Stop recording with `browser_record_stop` to get `videoPath` (`playthrough.mp4`) and `framesDir`.
5. Slice `videoPath` (or `framesDir`) into contact sheets and `manifest.json` using `extract_frames.mjs`:

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

## Step 5: Inspection, Fixes & Library Save

1. Read `report.json` (if any) and `frames/manifest.json`, then inspect gameplay `contactSheets` (`sheet_*.png`) and flagged `frames/frame_XXXX.png` with `view` for:
   - Broken controls, stuck/softlock spots, missing/misaligned crosshair or broken weapon hit/reload behaviour, broken restart state
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, **sustained FPS below 50** (optimize draw calls, shadows, LOD, particles/effects, or textures until `FPS >= 50` and verify real Graphics/Quality Settings work in medium/large games), and (for full games) unfair difficulty spikes.
2. Fix every defect found and re-verify (Showcase photos for asset/design fixes; code collision test suite for collision fixes; live gameplay video for runtime/gameplay/balance fixes) until clean.
3. Save new or improved reusable scripts to `<gamedev_library>` sorted by **feature, element, or physics topic** (`--scope feature`, `--scope element`, `--scope physics`, `--scope engine`, `--scope game`) with clear `--summary` and `--tags`. Never delete existing entries.
4. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
