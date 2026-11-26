# Prime GameDev — Game Craft & Genre Quality Standard

Use this guide when planning, implementing, or reviewing ANY game or mod so the result plays and looks like a real game rather than a generic prototype.

## 0. Scale to Scope (Match Depth to the Project)

Always calibrate effort to what the user is actually building:
- **Mini-game / 2D arcade / puzzle:** razor-sharp controls, crisp visuals & animation feedback, clean audio, smooth restart, persistent high score/settings, zero clutter — skip 3D shadow menus and 3D map flyovers.
- **Standalone medium / large full game (especially 3D):** apply the full universal standard below (art direction coherence, asset preloading, assembled map visual sweep, full enemy/entity lifecycles, live AI navigation, real Graphics/Quality Settings, `FPS >= 50`, progression/pacing).
- **Third-party game mod / plugin / addon:** apply standards strictly to the mod's own elements, colliders, animations, UI, and mechanics — never rewrite the host game or build duplicate host graphics settings menus.

---

## 1. Universal Game Quality Standard (All Genres)

1. **Coherent Art Direction (No Visual Mashup):**
   - Pick ONE visual direction and palette upfront in the plan (e.g. grounded realism, stylized low-poly, crisp pixel art) and unify downloaded/custom asset tones, roughness, scale, and lighting so every asset belongs in the same world.
2. **Assembled Map / Level Visual Sweep (Part of Stage A):**
   - In addition to photographing isolated elements on the Showcase scene, capture screenshots of the **assembled map/level** (overhead/flyover views + gameplay-camera views at key zones) and inspect them with `view` for:
     - floating props or objects sunk halfway into floors/walls;
     - gaps/cracks between walls, corners, and floor tiles exposing the void/skybox;
     - broken scale relative to the player/camera (waist-high doors, giant chairs, oversized props);
     - pitch-black unlit rooms, blown-out white hotspots, or detached shadows;
     - harsh repeating texture tiling across large floors/walls (break up with decals, lighting, props, or UV variation).
3. **Real Hit Reactions & Death / Destruction (Never Vanish in 1 Frame):**
   - Every attackable/interactive entity reacts visibly and audibly at the **impact point** (hit-flinch/recoil, sparks/blood/debris particles, sound, hitmarker/feedback).
   - Destroyed enemies/units/objects MUST play a real **death animation, fall/ragdoll, collapse, or genre-appropriate explosion/shatter** — NEVER delete the mesh in a single frame. Immediately disable collision and damage on death so corpses never block movement or hurt the player, keep remains briefly, then fade/pool cleanly within an object cap.
4. **Locomotion Feel, World Reactivity & Atmosphere (Anti-Empty-Box):**
   - Give movement tangible weight (brief responsive acceleration/deceleration rather than instant 0→100→0 snapping, smooth step-up over small stairs/ledges, stable slopes, soft landing camera/audio feedback, and crouch with an overhead ceiling check before standing).
   - Make the world react: highlight interactive objects with clear prompts (`[E] Open / Pick Up`), include genre-appropriate reactive/destructible props where fitting (crates, glass, explosive barrels, animated doors/switches), and make pickups smart (never consumed needlessly at full HP/ammo, always giving sound + visual HUD feedback).
   - Give levels readable landmarks, varied cover/silhouettes, directional + ambient/hemisphere lighting, skybox/background, and atmospheric depth (distance fog in 3D or parallax layers in 2D).
   - Add subtle ambient life (dust/sparks/weather particles, animated sky/water, flickering lights, patrolling units) and use lighting/composition to guide the player's eye toward objectives naturally.
5. **Camera Craft:**
   - Clamp 3D vertical pitch within `±85°` (never flip upside down); use spring-arm collision in third-person so cameras never clip through walls or let the player block the aim point.
   - In 2D / top-down / platformers, use smooth look-ahead, deadzone, and hard world-boundary clamping so the camera never exposes empty void outside the map.
   - Keep screen shake and hitstop brief and subtle so they sell impact without hurting readability or aim.
6. **Input Hygiene, Auto-Pause & Clean Resize:**
   - Clear all held input keys on window `blur`, `visibilitychange`, or pause (never leave the player running, steering, or firing after `Alt+Tab` or `Esc`).
   - Auto-pause gameplay and reset frame-delta timing (`lastTime`) when the tab/window loses focus so players never die off-screen or suffer a physics jump on return.
   - Keep mouse look frame-rate independent (`movementX/Y * sensitivity`, never multiplied by `dt`); handle `Pointer Lock` cleanly (`Esc` pauses & releases cursor; `Resume` cleanly re-locks).
   - Handle window `resize` cleanly (recompute viewport/aspect ratio so rendering never stretches), and add subtle backing/shadow behind HUD text so it stays readable against both bright skies and dark corners.
7. **Asset Preloading & Fair Spawning:**
   - Preload models, textures, HDRIs, and audio behind a clean loading state before gameplay starts (prevent falling through unloaded floors or models popping in mid-combat).
   - Spawn enemies/items only at validated clear positions (never inside walls or right behind the player); handle out-of-bounds falls with clean kill-zone/recovery so waves or objectives never softlock.
8. **Onboarding, Pacing, Progression & Persistence:**
   - Make the first 5 seconds clear without a wall of text: live styled title screen (NEVER bare browser `alert()` or unstyled default buttons), concise objective indicator, core loop with 2–3 distinct progression steps (new enemy types, mechanics, upgrades, or phases) and fair pacing (safe start -> escalation -> climax -> breather), clear win/loss summary, and working instant `Restart`.
   - Persist graphics/audio/sensitivity settings and best scores/progress (`localStorage` for web, config/save file for native).
9. **Pleasant Audio Feedback:**
   - Include clean, non-harsh audio for core actions (movement/footsteps/engine, attacks/interactions, hits, pickups, death/victory, UI clicks, light ambience) with volume control and a Mute toggle — never cheap piercing synthetic beeps.
10. **Long-Session Stability (Pooling & Zero Restart Leaks):**
    - Cap and pool dynamic entities (projectiles, shells, decals, particles, corpses, floating damage numbers) so sustained play stays `>= 50 FPS`.
    - Cleanly cancel old `requestAnimationFrame` loops, timers, physics worlds, and event listeners on `Restart` or level transitions (prevent 2x/3x speed bugs after repeated restarts).
11. **Game Anti-Slop Rules:**
    - Never ship raw cube/capsule placeholders for characters, enemies, vehicles, or weapons when real assets can be downloaded and adapted.
    - Never leave empty/non-functional settings buttons, dead HUD counters, or clipped weapon viewmodels poking through walls.

---

## 2. Genre-Specific Standards (Apply ONLY What Matches the Current Game)

- **Shooters (FPS / TPS / Top-Down Armed Action):**
  - High-contrast crosshair aligned with actual shot origin; true **ADS through the weapon's own iron sights / red-dot / optic** (aligning weapon & camera, NOT a raw FOV zoom with the gun off to the side); recoil + recovery + walk bob/sway; surface impact sparks/dust/decals; clean weapon switching & honest reload rules; headshots/weakspots; damage-direction indicator; viewmodel wall-clipping prevention.
  - Armed, animated enemies holding weapons in hand sockets, firing visible projectiles/tracers/muzzle flashes from the barrel, using obstacle avoidance / pathing + line-of-sight checks (never shooting through solid walls) + separation (never stacking onto identical coordinates).
- **Melee Action / RPG / Slasher:**
  - Readable swing arcs & enemy wind-up telegraphs, brief melee hitstop (30–50 ms), knockback, post-hit invulnerability frames (i-frames), stamina/dodge/block flow, visible weapons properly socketed in hands, full attack/hit/death animations, and loot/stat feedback.
- **Platformers / 2D Action:**
  - Coyote time (80–120 ms), jump buffering, variable jump height, responsive air control, smooth step-up / one-way platforms, slightly player-forgiving hazard hitboxes, and bounded smooth camera.
- **Racing / Driving / Vehicle Sims:**
  - Tangible weight, grip, acceleration, braking, drift/traction, speed-sensitive steering, strict checkpoint order (no lap-skipping), speedometer/lap/position HUD, tire marks/smoke/engine audio, dynamic chase camera with subtle speed FOV, and racing AI with path-following + collision avoidance.
- **Strategy / Tower Defense / Tactics:**
  - Valid/invalid placement preview (color + attack/effect range ring), clear resource/cost economy on HUD, crisp unit/tower selection & command feedback, game speed controls (`1x / 2x / Pause`), wave/path preview, and grid/navmesh pathfinding without stuck units.
- **Horror / Survival / Exploration / Interactive Adventure:**
  - Interactive object highlights & prompts (`[E] Open / Take`), working doors/switches/inventory/flashlight cone, smart pickups (never wasted at full HP/ammo/capacity), crouch with overhead ceiling check before standing, atmospheric audio cues, and readable spatial lighting.
- **Puzzles / Card / Board / Casual:**
  - Instant tactile click/drag feedback, clear legal-move highlighting, smooth move/combo/clear animations, Undo/Reset controls where appropriate, and guaranteed solvability for generated layouts.
