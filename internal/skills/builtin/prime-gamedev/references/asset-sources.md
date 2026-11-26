# GameDev Asset Sources Directory (Cross-Engine & Cross-Style)

Default rule: **download and adapt ready assets first** (native engine libraries -> `[Direct / API / No-Login]` web sources below -> free web search). Craft from scratch **only** when the user explicitly requests custom-made assets or no suitable asset exists.

**Do NOT waste turns trying to scrape login/Cloudflare walls with `fetch`/`download`.** Use `[Direct / API / No-Login]` sources first for automated downloads; use `[Account / Engine Plugin / Manual]` sources only via already-installed local engine packs/Vault/plugins or by offering them to the user.

---

## 1. Native Engine & Project Libraries (Check First)

- **Unreal Engine (UE4 / UE5)** `[Engine Plugin / Local Vault]`:
  - Check existing project `Content/`, local Vault / Megascans cache, `Starter Content`, and `Engine Content` first.
  - **Fab / Quixel Megascans** (`https://www.fab.com`) — entire 23,000+ AAA photogrammetry library free via Fab/Bridge (rocks, foliage, architecture, debris, props, PBR materials).
  - **MetaHuman** & Unreal's free **Game Animation Sample Project** (Manny + 500 AAA animations).
- **Unity** `[Engine Plugin / Account]`: installed packages + **Unity Asset Store Free** (`https://assetstore.unity.com/?free=true`).
- **Godot** `[Direct / Engine Library]`: project assets + **Godot Asset Library** (`https://godotengine.org/asset-library/asset`).

---

## 2. Direct / API / No-Login Web Sources (Best for Automated `download`)

Combine sources by category and visual style — **never pull every asset from one site**.

### Realistic 3D Models & Photogrammetry (`.glb`, `.gltf`, `.fbx`, `.obj`, `.blend`)
- **Poly Haven Models** (`https://polyhaven.com/models`, direct JSON API `https://api.polyhaven.com`) — 520+ CC0 photorealistic props, furniture, and nature assets, no login.
- **3D Valley** (`https://3dvalley.com`) — unified CC0 catalog of 9,500+ standardized `.glb` models from Poly Haven, AmbientCG, and Kenney.
- **Open Source 3D Assets** (`https://opensource3dassets.com`) — 990+ CC0 `.glb` models and environments.
- **Smithsonian 3D Digitization** (`https://3d.si.edu`) — CC0 museum-grade photogrammetry & LiDAR scans (statues, artifacts, spacecraft, fossils).

### 3D Weapons & Weapon Textures (`.glb`, `.fbx`, `.obj`)
- **Grab3D Weapons** (`https://grab3d.com/models/weapons/`) — CC0 swords, shields, armor, and action weapons (GLB/FBX/OBJ/BLEND), no login.
- **Cinevva Free 3D Weapons** (`https://app.cinevva.com/game-assets/free-3d-weapon-models`) — CC0 guns, swords, bows, and fantasy weapons in `.glb`, real-world metre scale ready to attach to hands.
- **Sloyd Weapons & Guns** (`https://www.sloyd.ai/free-3d-models/weapons-armor`, `https://www.sloyd.ai/free-3d-models/weapons-armor/guns-military`) — CC-BY 4.0 firearms, rifles, swords, and armor (GLB/FBX/OBJ).
- **Kenney 3D Blaster / Weapon Kits** (`https://kenney.nl/assets?t=3d`), **Quaternius Weapons** (`https://quaternius.com`), **Poly Pizza** (`https://poly.pizza`) — CC0 stylized/low-poly firearms & melee kits.

### 3D Character Animations & Auto-Rigging (`.glb`, `.fbx`, `.bvh`)
- **Mesh2Motion** (`https://mesh2motion.org`) — open-source Mixamo alternative (no account): 160+ CC0 human animations (idle, walk, run, jump, combat, climbing, zombie) + rigging for humans, animals, and monsters, exporting to **GLB & FBX**.
- **Quaternius & Kenney Pre-Animated Characters** (`https://quaternius.com`, `https://kenney.nl/assets?t=3d`) — CC0 characters with dozens of built-in game animations (GLB/FBX).
- **KayKit Character Animations** (`https://kaylousberg.itch.io`) — CC0 rigged fantasy/sci-fi characters and shared animation libraries (FBX/glTF).
- **Gobkit Freebies** (`https://gobkit.com/freebies`) — 400+ CC0 rigged/animated `.glb` characters & props, no login.
- **CMU Mocap Converted Packs** (`https://rancidmilk.itch.io/free-character-animations`) — public-domain mocap converted to ready-to-use FBX/glTF.

### Stylized & Low-Poly 3D Models (`.glb`, `.fbx`, `.obj`)
- **Grab3D** (`https://grab3d.com/free-gltf-models/`) — 2,500+ CC0 models with embedded PBR textures, real-world scale, no login.
- **Polyfork** (`https://polyfork.dev` / `https://github.com/lucas-martinic/polyfork-catalog`) — 3,600+ free low-poly `.glb` models across 38 kits with a direct `catalog.json`, no login.
- **Quaternius** (`https://quaternius.com`), **Kenney 3D** (`https://kenney.nl/assets?t=3d`), **Poly Pizza** (`https://poly.pizza`), **KayKit** (`https://kaylousberg.itch.io`), **Itch.io Free GLB/FBX Packs** (`https://itch.io/game-assets/free/tag-glb`, `https://itch.io/game-assets/free/tag-3d/tag-fbx/tag-weapons`).

### Realistic Seamless PBR Textures (Color / Normal / Roughness / Metallic / AO)
- **AmbientCG** (`https://ambientcg.com`) — 2,000+ CC0 PBR materials, HDRIs, and models (1K–8K), no login.
- **Poly Haven Textures** (`https://polyhaven.com/textures`, `https://api.polyhaven.com`) — 860+ CC0 PBR surfaces (1K–8K), no login.
- **3D Valley Textures** (`https://3dvalley.com/textures`) — unified CC0 search across 3,800+ PBR sets.
- **CGBookcase** (`https://www.cgbookcase.com/textures`), **ShareTextures** (`https://www.sharetextures.com`), **TextureCan** (`https://www.texturecan.com`), **Meshy Free Textures** (`https://www.meshy.ai/textures`).

### Stylized / Hand-Painted Seamless PBR Textures
- **StylizedTextures.com** (`https://stylizedtextures.com`) — 100% free CC0 4K stylized PBR packs, no sign-up.
- **FreeStylized.com** (`https://freestylized.com`) — free stylized textures, skyboxes, and 3D packs.
- **3DTextures.me** (`https://3dtextures.me`) — CC0 seamless stylized & realistic PBR sets.

### 2D Sprites, Tilesets, HUD, Crosshairs, HDRI & Audio
- **2D Sprites & Pixel Art:** **Kenney 2D** (`https://kenney.nl/assets?t=2d`), **GameArt2D Freebies** (`https://www.gameart2d.com/freebies.html`), **OpenGameArt** (`https://opengameart.org`), **DOTOWN** (`https://dotown.maeda-design-room.net`), **Itch.io CC0 Pixel Art** (`https://itch.io/game-assets/free/tag-cc0/tag-pixel-art`), **CraftPix Freebies** (`https://craftpix.net/freebies/`).
- **HUD, UI & Crosshairs:** **Game-icons.net** (`https://game-icons.net` — 4,000+ CC-BY SVG/PNG icons), **Kenney UI & Crosshairs** (`https://kenney.nl/assets?t=ui`).
- **HDRI Skies:** **Poly Haven HDRIs** (`https://polyhaven.com/hdris`), **AmbientCG HDRIs** (`https://ambientcg.com`).
- **Audio (SFX/Music):** **Kenney Audio** (`https://kenney.nl/assets?t=audio`), **OpenGameArt Audio** (`https://opengameart.org/content/free-sounds`), **Itch.io CC0 Music/SFX** (`https://itch.io/game-assets/free/tag-cc0/tag-music`).

---

## 3. Account / Engine-Plugin / Manual Sources (Do Not Scrape Login Pages)

Use via local installed libraries/plugins or offer to the user — do NOT loop on login/Cloudflare pages:
- **Fab / Quixel Megascans** (`https://www.fab.com`) — AAA photogrammetry & Unreal packs.
- **Adobe Mixamo** (`https://www.mixamo.com`) — thousands of humanoid mocap clips & auto-rigger (FBX).
- **CGTrader Free** (`https://www.cgtrader.com/free-3d-models`), **TurboSquid Free** (`https://www.turbosquid.com/Search/3D-Models/free`), **Sketchfab Free CC** (`https://sketchfab.com/features/free-3d-models`), **Polycam** (`https://polycam.ai`).
- **Meshy Free Weapons & Military** (`https://www.meshy.ai/category/weapons-military`).
- **MoCap Online Free** (`https://mocaponline.com/blogs/mocap-news/free-3d-animations`), **Reallusion AccuRIG / ActorCore** (`https://actorcore.reallusion.com`), **Free3D Rigged FBX** (`https://free3d.com/3d-models/rigged-fbx-characters`).
- **Freesound** (`https://freesound.org`).

---

## 4. Platform Format, Adaptation & Animation Fitting Rules

- **Desktop / Native engines (Unreal, Unity, Godot, Bevy/Rust, C++):** prefer engine-native packs / **FBX** (Unreal/Unity) or **GLB/glTF** (Godot/Bevy/Blender) and 2K–4K PBR maps.
- **Browser / WebGL (Three.js, Babylon, Phaser):** prefer lightweight **GLB/glTF** and 512px–1K `.webp`/`.png`/`.jpg` textures.
- **Always adapt downloaded assets:** normalize real-world scale (metres), forward axis, weapon grip/muzzle pivot in character hands, PBR maps, and UV seams.
- **Always fit downloaded animations to the specific character model:** retarget bones/skeleton cleanly, fix foot-ground contact (no floating/sinking), root motion & locomotion speed (no foot sliding), limb twisting, hand-socket weapon alignment, and blend transitions. **If an animation looks bad or even slightly awkward, edit/fix it (keyframes, bone weights, retarget curves, IK, speed/blends) or swap to a cleaner clip** and verify on the Showcase scene.
