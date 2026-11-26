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
7. **Performance Budget (`>= 50 FPS`) & Visual/Graphics Settings (Plan Upfront):**
   - Design for smooth performance: target **at least 50 FPS** during active gameplay (use instancing/batching, object pooling for projectiles/particles, frustum culling/LOD, bounded shadow-casting lights, and appropriately sized textures).
   - For any **standalone medium or large full game** (especially 3D or visually rich projects), plan and implement **real working Graphics / Quality Settings** accessible from Main Menu and Pause Menu: overall quality preset (`Low / Medium / High`), shadow quality (`Off / Low / Medium / High`), draw distance / LOD / effects & particle density, post-processing / antialiasing, and render scale / DPR cap (plus audio & mouse sensitivity where applicable). Every setting MUST actually change the renderer/performance — never build decorative non-functional toggles. **Do NOT build a graphics settings menu for a mod/plugin/addon to an existing third-party game** (the host game already owns graphics settings — for mods, simply keep the mod's own features from dropping FPS below 50).

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
5. **Sourcing, adapting & polishing textures, 3D models (realistic & stylized), 3D weapons, character animations & audio (DEFAULT = DOWNLOAD & ADAPT, NEVER RELY ON ONE SITE):**
   - **Default rule — download first, craft from scratch ONLY on request or gap:** unless the user explicitly asks you to create models, weapons, textures, or animations from scratch, **by default download ready assets** instead of hand-building crude primitives:
     1. First check and reuse **native engine and project asset libraries**:
        - **Unreal Engine (UE4/UE5):** existing project `Content/` & Vault packs, `Starter Content` / `Engine Content`, **Fab / Quixel Megascans** (`https://www.fab.com` — entire 23,000+ AAA photogrammetry library free on Fab: rocks, vegetation, architecture, debris, props, PBR surfaces), **MetaHuman**, and Unreal's free **Game Animation Sample Project** (Manny + 500 AAA animations).
        - **Unity:** installed packages + **Unity Asset Store Free** (`https://assetstore.unity.com/?free=true`).
        - **Godot:** project assets + **Godot Asset Library** (`https://godotengine.org/asset-library/asset`).
     2. Next pull from the **curated category directory below**, combining sources by engine, style, and asset category (never pulling every asset from a single site).
     3. You may also **search the web freely** (`web_search`, `agentic_fetch`, `fetch`, `download`) whenever you need a specific realistic model, weapon, texture, or animation not in the curated list (verify license & format).
     4. Craft assets yourself (via `generate_image`, procedural shaders/canvas scripts, or custom geometry) **only when the user explicitly asks you to make them**, or when no suitable asset exists after searching.
   - **Always adapt & fix downloaded models, weapons, textures, and animations (ZERO "good enough" tolerance):**
     - **Models, weapons & textures:** fix real-world scale (metres), forward orientation & weapon grip/muzzle pivot so weapons sit properly in character hands, missing/broken PBR maps (Albedo/Normal/Roughness/Metallic/AO), UV seams, and texture resolution for target performance.
     - **Downloaded animations — ALWAYS fit to the specific character model and fix if even slightly off:** retarget bones/skeleton cleanly to the model, fix foot-ground contact (no floating or sinking feet), root motion / locomotion speed matching (no foot sliding), shoulder/wrist/limb twisting, weapon hand-socket alignment, and loop/blend transitions. **If an animation looks bad or even slightly awkward, edit/fix the animation (keyframes, bone weights, retarget curves, IK, speed/blends) or replace it with a cleaner clip** and inspect key poses/motion until it looks genuinely smooth and natural.
   - **Match format and resolution to the target platform:**
     - **Desktop / Native engines (Unreal, Unity, Godot, Bevy/Rust, C++):** prefer engine-native packs / **FBX** (Unreal/Unity) or **GLB/glTF** (Godot/Bevy/Blender) and 2K–4K PBR maps; verify humanoid skeleton compatibility (Unreal Mannequin / Unity Humanoid / Mixamo / Mesh2Motion).
     - **Browser / WebGL (Three.js, Babylon, Phaser):** prefer lightweight **GLB/glTF** and 512px–1K `.webp`/`.png`/`.jpg` textures instead of heavy 8K files.
   - **Curated Asset Source Directory by Style & Category (starting points — expand via web search when needed):**
     - **Realistic 3D models & photogrammetry scans** (`.glb`, `.gltf`, `.fbx`, `.obj`, `.blend`):
       - **Fab / Quixel Megascans** (`https://www.fab.com`) — AAA photorealistic photogrammetry props, architecture, foliage, rocks, debris, and PBR surfaces.
       - **Poly Haven Models** (`https://polyhaven.com/models`) — 520+ CC0 photorealistic props, furniture, and nature assets (no login, direct API, GLB/FBX/BLEND).
       - **3D Valley** (`https://3dvalley.com`) — unified CC0 search across 9,500+ standardized `.glb` models from Poly Haven, ambientCG, and Kenney.
       - **Open Source 3D Assets** (`https://opensource3dassets.com`) — 990+ CC0 `.glb` realistic/game models and environments.
       - **Smithsonian 3D Digitization** (`https://3d.si.edu`) — CC0 museum-grade photogrammetry & LiDAR scans (statues, artifacts, spacecraft, fossils) in GLB/OBJ.
       - **CGTrader Free** (`https://www.cgtrader.com/free-3d-models`), **TurboSquid Free** (`https://www.turbosquid.com/Search/3D-Models/free`), **Sketchfab Free Photogrammetry / CC0** (`https://sketchfab.com/features/free-3d-models`), and **Polycam** (`https://polycam.ai`) — realistic vehicles, buildings, props, and real-world scans (verify license per asset).
     - **3D weapons (guns, rifles, pistols, swords, bows, melee, sci-fi & weapon textures)** (`.glb`, `.fbx`, `.obj`):
       - **Grab3D Weapons** (`https://grab3d.com/models/weapons/`) — CC0 swords, shields, armor, and action weapons in GLB/FBX/OBJ/BLEND, no login.
       - **Cinevva Free 3D Weapons** (`https://app.cinevva.com/game-assets/free-3d-weapon-models`) — CC0 guns, swords, bows, and fantasy weapons in `.glb`, scaled in metres and ready to attach to character hands.
       - **Sloyd Weapons & Guns** (`https://www.sloyd.ai/free-3d-models/weapons-armor` and `https://www.sloyd.ai/free-3d-models/weapons-armor/guns-military`) — CC-BY 4.0 firearms, rifles, swords, and armor in GLB/FBX/OBJ.
       - **Meshy Free Weapons & Military** (`https://www.meshy.ai/category/weapons-military`) — large catalog of weapons, firearms, blades, and tactical gear in FBX/GLB/OBJ.
       - **Itch.io Free 3D Weapons (FBX/GLB)** (`https://itch.io/game-assets/free/tag-3d/tag-fbx/tag-weapons`) — low-poly and modular firearm/sword packs.
       - **Kenney 3D Blaster/Weapon Kits** (`https://kenney.nl/assets?t=3d`), **Quaternius Weapons** (`https://quaternius.com`), **Poly Pizza** (`https://poly.pizza`), and **CGTrader / Sketchfab Free CC** — stylized and realistic firearms/melee weapons.
     - **3D character animations & auto-rigging (idle, walk, run, jump, crouch, shoot, reload, melee, hit, death)** (`.fbx`, `.glb`, `.bvh`):
       - **Mesh2Motion** (`https://mesh2motion.org`) — open-source Mixamo alternative (no account needed): 160+ CC0 human animations (locomotion, combat, climbing, zombies) plus rigging for humans, animals, and monsters, exporting directly to **GLB and FBX**.
       - **Adobe Mixamo** (`https://www.mixamo.com`) — largest free humanoid mocap library (thousands of idle/walk/run/shoot/reload/sword/death clips) + auto-rigger for Unreal, Unity, Godot, and Blender (FBX).
       - **Quaternius & Kenney Pre-Animated Characters** (`https://quaternius.com`, `https://kenney.nl/assets?t=3d`) — CC0 characters shipping with dozens of built-in game animations in GLB & FBX.
       - **KayKit Character Animations** (`https://kaylousberg.itch.io`) — CC0 rigged fantasy/sci-fi characters and shared animation libraries in FBX/glTF.
       - **Gobkit Freebies** (`https://gobkit.com/freebies`) — 400+ CC0 rigged/animated `.glb` game characters and props, no login.
       - **CMU Mocap Converted Packs** (`https://rancidmilk.itch.io/free-character-animations`) — public-domain CMU motion capture converted to ready-to-use FBX/glTF humanoid animations.
       - **MoCap Online Free Packs** (`https://mocaponline.com/blogs/mocap-news/free-3d-animations`), **Reallusion AccuRIG / ActorCore Free** (`https://actorcore.reallusion.com`), and **Free3D Rigged FBX** (`https://free3d.com/3d-models/rigged-fbx-characters`) — FBX mocap & auto-rigging for Unreal/Unity/Blender.
     - **Stylized & low-poly 3D models (characters, environments, props, vehicles)** (`.glb`, `.fbx`, `.obj`):
       - **Grab3D Free 3D Models** (`https://grab3d.com/free-gltf-models/`) — 2,500+ CC0 GLB/FBX/OBJ models with embedded PBR textures, no login.
       - **Polyfork** (`https://polyfork.dev` / `https://github.com/lucas-martinic/polyfork-catalog`) — 3,600+ free low-poly `.glb` models across 38 themed sets with a direct `catalog.json`, no login.
       - **Quaternius** (`https://quaternius.com`), **Kenney 3D** (`https://kenney.nl/assets?t=3d`), **Poly Pizza** (`https://poly.pizza`), **KayKit** (`https://kaylousberg.itch.io`), and **Itch.io Free GLB/FBX Packs** (`https://itch.io/game-assets/free/tag-glb`).
     - **Realistic seamless PBR textures & materials** (floors, walls, brick, concrete, asphalt, wood, metal, stone, grass, fabric — color/normal/roughness/AO maps):
       - **AmbientCG** (`https://ambientcg.com`) — 2,000+ CC0 PBR materials, HDRIs, and models (1K–8K), no login.
       - **Poly Haven Textures** (`https://polyhaven.com/textures`) — 860+ high-craft CC0 PBR surfaces (1K–8K), no login (direct API at `api.polyhaven.com`).
       - **3D Valley** (`https://3dvalley.com/textures`) — unified CC0 search across 3,800+ PBR materials from Poly Haven, AmbientCG, CGBookcase, and TextureCan.
       - **CGBookcase** (`https://www.cgbookcase.com/textures`), **ShareTextures** (`https://www.sharetextures.com`), **TextureCan** (`https://www.texturecan.com`), and **Meshy Free Textures** (`https://www.meshy.ai/textures`) — CC0 seamless PBR sets, no login.
     - **Stylized / hand-painted seamless PBR textures:**
       - **StylizedTextures.com** (`https://stylizedtextures.com`) — 100% free CC0 seamless stylized 4K PBR packs, no sign-up.
       - **FreeStylized.com** (`https://freestylized.com`) — free stylized textures, skyboxes, and 3D packs.
       - **3DTextures.me** (`https://3dtextures.me`) — CC0 seamless stylized & realistic PBR textures.
     - **2D sprites, pixel art, tilesets & backgrounds** (`.png`, `.svg`, sprite sheets):
       - **Kenney 2D** (`https://kenney.nl/assets?t=2d`), **Itch.io CC0 Pixel Art & Sprites** (`https://itch.io/game-assets/free/tag-cc0/tag-pixel-art`), **GameArt2D Freebies** (`https://www.gameart2d.com/freebies.html`), **OpenGameArt** (`https://opengameart.org`), **CraftPix Freebies** (`https://craftpix.net/freebies/`), and **DOTOWN** (`https://dotown.maeda-design-room.net`).
     - **HUD, UI panels, crosshairs & icons:**
       - **Game-icons.net** (`https://game-icons.net`) — 4,000+ CC-BY SVG/PNG icons for weapons, ammo, skills, inventory, and reticles.
       - **Kenney UI & Crosshair Packs** (`https://kenney.nl/assets?t=ui`) — CC0 HUD frames, buttons, cursors, and crosshair packs.
     - **Skyboxes, lighting (HDRI) & audio (SFX/music):**
       - **Poly Haven HDRIs** (`https://polyhaven.com/hdris`) and **AmbientCG HDRIs** (`https://ambientcg.com`) — CC0 skyboxes and IBL lighting for any 3D engine.
       - **Kenney Audio** (`https://kenney.nl/assets?t=audio`), **Freesound** (`https://freesound.org`), **OpenGameArt Audio** (`https://opengameart.org/content/free-sounds`), and **Itch.io CC0 Music/SFX** (`https://itch.io/game-assets/free/tag-cc0/tag-music`).
   - **Every engine-library, downloaded, and custom asset (including 3D weapons & animated characters) MUST be placed on the Showcase scene and photographed from multiple sides.** If any asset or animation looks ugly, cheap, mismatched, wrongly scaled, or even slightly awkward, fix, adapt, or remake it yourself and re-shoot the Showcase.
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
   - Runtime/console/WebGL/shader errors, 404 assets, `NaN` physics, **sustained FPS below 50** (optimize draw calls, shadows, LOD, particles/effects, or textures until `FPS >= 50` and verify real Graphics/Quality Settings work in medium/large games), and (for full games) unfair difficulty spikes.
2. Fix every defect found and re-verify (Showcase photos for asset/design fixes; code collision test suite for collision fixes; live gameplay video for runtime/gameplay/balance fixes) until clean.
3. Save new or improved reusable scripts to `<gamedev_library>` sorted by **feature, element, or physics topic** (`--scope feature`, `--scope element`, `--scope physics`, `--scope engine`, `--scope game`) with clear `--summary` and `--tags`. Never delete existing entries.
4. Remove temporary video/frame run folders from `os.tmpdir()` when finished unless the user requested to keep them.
