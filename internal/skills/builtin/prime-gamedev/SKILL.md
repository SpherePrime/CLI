---
name: prime-gamedev
description: Game design/engineering baseline (balance plan, ideal collisions, weapons & crosshair, game feel, HUD), dedicated multi-angle Asset Showcase scene visual QA for all custom and downloaded elements, deterministic code-level collision testing across all sides, live gameplay bot video recording, texture/model sourcing, and sorted script library for games and mods.
user-invocable: false
disable-model-invocation: true
---

# Prime GameDev Engineering & QA

Design and build games right from the start (concrete balance plan, airtight collisions with zero texture fall-throughs, mandatory crosshair and feedback whenever weapons exist, clean controls, fair enemies, readable HUD), then verify every game or mod using three focused, non-overlapping QA stages:
- **Stage A — Visual QA on a dedicated Showcase scene:** place every created AND downloaded visual element separately (characters, enemies, weapons, items, props, structures, effects, mod additions) in a standalone studio-lit showcase scene, run a fast script that photographs each element from multiple angles, inspect the photos with `view`, and fix/remake anything that looks wrong.
- **Stage B — Deterministic collision & physics tests in code (NO in-game teleporting):** write and run real automated code tests verifying that every solid object, wall, floor, prop, door, and item actually has a collider matching its rendered visual bounds, blocks movement from all sides (all 4 sides in 2D / all 6 faces in 3D), handles corners and seams cleanly, prevents high-speed tunneling, and enforces map bounds and 3D camera collision.
- **Stage C — Live gameplay bot video check:** run a real live gameplay bot inside the game under window/tab-scoped video recording to verify controls, menu/pause/restart flow, combat/crosshair, runtime errors, softlocks, FPS, and difficulty/balance — without teleporting around the map and without using gameplay video to judge individual asset appearance.

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
   - Keep collision world definitions and step resolution decoupled from rendering so automated code tests can run directly against the real collision engine.
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
2. **Strict three-way separation (no teleport hacks):**
   - **Stage A (Showcase visuals):** all visuals (custom AND downloaded models, sprites, textures, weapons, reticles, characters, props, structures, effects) are verified on a dedicated standalone **Showcase scene** via fast multi-angle photo capture and `view` inspection.
   - **Stage B (Code collision tests):** all object colliders, visual-vs-hitbox alignment, all-sides blocking (4 sides in 2D / 6 faces in 3D), corners, seams, tunneling, bounds, and 3D camera walls are verified by **deterministic automated tests in code** — NEVER by teleporting a bot around the gameplay map.
   - **Stage C (Live gameplay video):** a live bot in the built-in browser / target game window verifies real controls, gameplay loop, combat/crosshair, menu/restart, runtime errors, softlocks, and difficulty/balance.
3. **Never hijack the user's mouse or keyboard and never record the full computer screen.**
   - **Browser games & web tasks (when `browser_*` built-in browser tools are available):** do EVERYTHING completely inside Prime Desktop's built-in browser in the background (`browser_navigate`, `browser_record_start`, `browser_run`, `browser_press_key`, `browser_evaluate`, `browser_screenshot`, `browser_record_stop`). Do NOT open external Chrome/Playwright when built-in browser tools are available, and never move the user's physical cursor.
   - **Native/desktop games (or CLI-only web fallback without `browser_*`):** record strictly the target game window or isolated headless browser viewport (`native_bot_runner.mjs` / `web_bot_runner.mjs`), never the full computer desktop.
4. **Own Game vs. Mod Scope:**
   - **Full game you are building:** put all game elements on the Showcase scene for multi-angle visual inspection, cover all world objects in code collision tests, and verify gameplay loop, weapons/crosshair, UI/HUD, win/loss/restart states, and difficulty balance on the live bot run.
   - **Mod / plugin / addon for an existing third-party game:** put **only the mod's own elements** on the Showcase scene, and test only the mod's own colliders and functionality — do NOT audit or try to fix unrelated bugs in the base game.
5. **Sourcing & polishing textures, sprites, 3D models, animations & audio (pick the right source for the engine, style & category — NEVER pull everything from one site):**
   - Unless the user asked you to craft assets yourself, **first check and reuse native engine and project asset libraries**:
     - **Unreal Engine (UE4/UE5):** check existing project `Content/`, local Vault/Megascans packs, `Starter Content` / `Engine Content`, **Fab / Quixel Megascans** (`https://www.fab.com`), **MetaHuman**, and Unreal's free **Game Animation Sample Project** before building primitives or fetching unrelated third-party meshes.
     - **Unity:** check installed packages and **Unity Asset Store Free** (`https://assetstore.unity.com/?free=true`).
     - **Godot:** check project assets and **Godot Asset Library** (`https://godotengine.org/asset-library/asset`).
   - **Match format and resolution to the target platform:**
     - **Desktop / Native engines (Unreal, Unity, Godot, Bevy/Rust, C++):** prefer **FBX** (Unreal/Unity) or **GLB/glTF** (Godot/Bevy/Blender) and 2K–4K PBR maps; verify humanoid skeleton compatibility (Unreal Mannequin / Unity Humanoid / Mixamo).
     - **Browser / WebGL (Three.js, Babylon, Phaser):** prefer lightweight **GLB/glTF** and 512px–1K `.webp`/`.png`/`.jpg` textures instead of heavy 8K files.
   - **Diversify web sources by asset category and visual style** (search via `web_search` / `agentic_fetch` / `fetch`, prefer direct no-login CC0 downloads via `download` into the project's asset folder):
     - **Realistic seamless PBR textures & materials** (floors, walls, brick, concrete, asphalt, wood, metal, stone, grass, fabric — color/normal/roughness/AO maps):
       - **AmbientCG** (`https://ambientcg.com`) — 2,000+ CC0 PBR materials, HDRIs, and models (1K–8K), no login.
       - **Poly Haven Textures** (`https://polyhaven.com/textures`) — 860+ high-craft CC0 PBR surfaces (1K–8K), no login.
       - **3D Valley** (`https://3dvalley.com/textures`) — unified CC0 search across 3,800+ PBR materials from Poly Haven, AmbientCG, CGBookcase, and TextureCan.
       - **CGBookcase** (`https://www.cgbookcase.com/textures`) — CC0 seamless PBR materials (roads, walls, floors, metals, fabrics).
       - **ShareTextures** (`https://www.sharetextures.com`) — CC0 architectural and natural PBR materials up to 4K.
       - **TextureCan** (`https://www.texturecan.com`) — CC0 PBR textures (concrete, wood, metal, leather, foliage).
       - **Meshy Free Textures** (`https://www.meshy.ai/textures`) — 600+ CC0 seamless PBR textures up to 4K, no login.
       - **Fab / Quixel Megascans** (`https://www.fab.com`) — photorealistic scans and AAA PBR materials.
     - **Stylized / hand-painted seamless PBR textures** (for cartoon, fantasy, low-poly, indie 3D worlds — do NOT paste gritty photoreal concrete onto stylized games):
       - **StylizedTextures.com** (`https://stylizedtextures.com`) — 100% free CC0 seamless stylized 4K PBR packs (Basecolor/Normal/Roughness/Metallic/AO/Height), no sign-up.
       - **FreeStylized.com** (`https://freestylized.com`) — free stylized textures, skyboxes, and 3D packs tailored for stylized 3D games.
       - **3DTextures.me** (`https://3dtextures.me`) — CC0 seamless stylized & realistic PBR textures (stone, tiles, brick, sci-fi panels, terrain).
     - **3D models, weapons, props, vehicles & modular environments** (`.fbx`, `.glb`, `.gltf`, `.obj`, `.uasset`, `.unitypackage`):
       - **Grab3D** (`https://grab3d.com/free-gltf-models/`) — 2,500+ CC0 models in GLB, FBX, OBJ, BLEND, USDZ with embedded PBR textures and real-world scale, no login.
       - **Polyfork** (`https://polyfork.dev` / `https://github.com/lucas-martinic/polyfork-catalog`) — 3,600+ free low-poly `.glb` models & 38 themed kits with `catalog.json`, no account or API key.
       - **Quaternius** (`https://quaternius.com`) — stylized & animated 3D packs in FBX, OBJ, Blend, and GLB (characters, monsters, weapons, medieval, sci-fi, nature).
       - **Kenney 3D** (`https://kenney.nl/assets?t=3d`) — clean CC0 low-poly kits (weapons, cars, cities, dungeons, space, characters) in FBX/GLB/OBJ.
       - **Poly Pizza** (`https://poly.pizza`) — thousands of lightweight low-poly `.glb`/`.gltf` weapons, props, buildings, and vehicles.
       - **KayKit** (`https://kaylousberg.itch.io`) — stylized 3D packs (dungeons, knights, skeletons, weapons, space, props) in FBX and glTF for Unreal, Unity, and Godot.
       - **Poly Haven Models** (`https://polyhaven.com/models`) — realistic CC0 3D props and environment assets (FBX/GLB/Blend).
       - **CGTrader Free** (`https://www.cgtrader.com/free-3d-models`) and **Sketchfab Free Downloadable** (`https://sketchfab.com/features/free-3d-models`) — realistic/hero weapons, vehicles, props, and characters in FBX, `.uasset`, `.unitypackage`, GLB (filter by free CC license).
       - **Itch.io 3D & GLB Packs** (`https://itch.io/game-assets/free/tag-glb`) — indie PSX/retro/low-poly 3D packs (verify license on page).
     - **Rigged 3D characters & motion-capture animations** (for Unreal, Unity, Godot, Blender, Web):
       - **Adobe Mixamo** (`https://www.mixamo.com`) — free rigged characters, thousands of mocap animations (locomotion, combat, reactions), and auto-rigger for static meshes.
       - **Gobkit Freebies** (`https://gobkit.com/freebies`) — 400+ CC0 `.glb` models including 388 rigged/animated low-poly characters, no account.
       - **Free3D Rigged FBX** (`https://free3d.com/3d-models/rigged-fbx-characters`) — free rigged humanoid/creature FBX models.
       - **MoCap Online Free** (`https://mocaponline.com/blogs/mocap-news/free-3d-animations`) — free FBX motion capture animations for Unreal, Unity, and Blender.
     - **2D sprites, pixel art, tilesets & backgrounds** (`.png`, `.svg`, sprite sheets):
       - **Kenney 2D** (`https://kenney.nl/assets?t=2d`) — 100% CC0 clean vector and pixel-art platformer, top-down shooter, RPG, and space sprite packs.
       - **Itch.io CC0 Pixel Art & Sprites** (`https://itch.io/game-assets/free/tag-cc0/tag-pixel-art`) — rich indie pixel-art characters, tilesets, animations, and VFX (verify license per pack).
       - **GameArt2D Freebies** (`https://www.gameart2d.com/freebies.html`) — CC0 polished 2D character sprites, tilesets, and GUI kits.
       - **OpenGameArt** (`https://opengameart.org`) — community archive of retro sprites, tilesets, VFX, and tileable textures (filter CC0 / CC-BY).
       - **CraftPix Freebies** (`https://craftpix.net/freebies/`) — 2D characters, parallax backgrounds, tilesets, and effects.
       - **DOTOWN** (`https://dotown.maeda-design-room.net`) — 700+ crisp Nintendo-style pixel-art sprites free for commercial use.
     - **HUD, UI panels, crosshairs & icons:**
       - **Game-icons.net** (`https://game-icons.net`) — 4,000+ CC-BY SVG/PNG icons for weapons, ammo, skills, inventory, and reticles.
       - **Kenney UI & Crosshair Packs** (`https://kenney.nl/assets?t=ui`) — CC0 HUD frames, buttons, cursors, and crosshair packs.
     - **Skyboxes, lighting (HDRI) & audio (SFX/music):**
       - **Poly Haven HDRIs** (`https://polyhaven.com/hdris`) and **AmbientCG HDRIs** (`https://ambientcg.com`) — CC0 skyboxes and IBL lighting for any 3D engine.
       - **Kenney Audio** (`https://kenney.nl/assets?t=audio`), **Freesound** (`https://freesound.org`), **OpenGameArt Audio** (`https://opengameart.org/content/free-sounds`), and **Itch.io CC0 Music/SFX** (`https://itch.io/game-assets/free/tag-cc0/tag-music`) — footsteps, gunshots, impacts, ambience, UI SFX, and music loops.
   - Craft assets yourself (via `generate_image`, procedural shaders/canvas scripts, or asset builders) whenever the user explicitly asks you to make them, or when no suitable asset exists in native libraries or online.
   - **Every engine-library, downloaded, and custom asset (including weapons) MUST be placed on the Showcase scene and photographed from multiple sides.** If any asset looks ugly, cheap, mismatched in style/scale, or has UV/seam/geometry defects, fix or remake it yourself and re-shoot the Showcase.
6. **Reuse built-in presets & save context with `<gamedev_library>`.** Built-in reusable presets already ship under `<skill_directory>/scripts/presets/` (`showcase_capture_probe.mjs`, `collision_suite_probe.mjs`, `weapon_crosshair_probe.mjs`, `physics_bounds_probe.mjs`, `balance_and_state_probe.mjs`) and are automatically indexed by `library.mjs lookup`. Always query `library.mjs lookup` first, read only the 1–2 matching scripts, and save new or improved reusable scripts into the proper sorted folder (`features/`, `physics/`, `engines/`, `elements/`, `games/`) without deleting existing ones.
7. **Fix every bug found and re-verify:** fix visual defects found on Showcase photos (re-shoot Showcase), fix collision flaws found by code collision tests (re-run test suite), and fix gameplay/runtime/balance bugs (re-run live bot video) until clean.

## Step 1: Lookup Relevant Presets & Library Scripts (Context-Efficient)

```bash
node "<skill_directory>/scripts/library.mjs" lookup --feature "<feature>" --physics "<topic>" --engine "<engine>" --game "<game-slug>" --limit 6
```

Read **only** the 1–2 matching scripts returned by `lookup` (for example `presets/showcase_capture_probe.mjs`, `presets/collision_suite_probe.mjs`, `presets/weapon_crosshair_probe.mjs`, or saved library entries). Never read unrelated folders.

## Step 2 (Stage A): Dedicated Asset Showcase Scene & Multi-Angle Photo Capture

1. Build a lightweight standalone Showcase mode/scene (for example `?showcase=1` in web games or a dedicated showcase scene/flag in native engines) with clean neutral lighting where **every standalone created and downloaded element** is isolated (player, enemies, weapons, reticles, items, props, structures, effects, mod additions).
2. Expose a lightweight inspection hook (`window.__SHOWCASE__.listElements()` and `window.__SHOWCASE__.focusElement(id, angle)` for web, or CLI/IPC cycling for native) supporting multiple sides (`front`, `right`, `back`, `left`, `quarter` for 3D; close-up + key animation/weapon states for 2D).
3. Run a fast automated photo pass (`presets/showcase_capture_probe.mjs`, or `browser_run` + `browser_screenshot` in the built-in browser, or native engine frame dump) to photograph every element from all sides quickly without navigating the gameplay map.
4. Inspect **every** generated showcase photo / contact sheet with `view` (when image viewing is supported) for ugly/broken models, weapon design flaws, mismatched style/scale, bad UVs/seams, missing textures, z-fighting, or broken shading. Fix or remake any flawed element and re-shoot until every asset looks clean.

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
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, and (for full games) unfair difficulty spikes.
2. Fix every defect found and re-verify (Showcase photos for asset/design fixes; code collision test suite for collision fixes; live gameplay video for runtime/gameplay/balance fixes) until clean.
3. Save new or improved reusable scripts to `<gamedev_library>` sorted by **feature, element, or physics topic** (`--scope feature`, `--scope element`, `--scope physics`, `--scope engine`, `--scope game`) with clear `--summary` and `--tags`. Never delete existing entries.
4. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
