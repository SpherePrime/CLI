package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type gameDevContextKey struct{}

func WithGameDev(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, gameDevContextKey{}, enabled)
}

func GameDevFromContext(ctx context.Context) (bool, bool) {
	if ctx == nil {
		return false, false
	}
	v, ok := ctx.Value(gameDevContextKey{}).(bool)
	return v, ok
}

const (
	gameDevEngineeringBaseline = "read prime://skills/prime-gamedev/references/game-craft.md (calibrate depth via Scale to Scope: mini-game vs. standalone medium/large full game vs. third-party mod, applying ONLY genre rules matching the current game — shooters, melee/RPG, platformers, racing, strategy/TD, horror/survival, puzzles); draft a concrete balance plan before coding (coherent art direction, progression/pacing, speeds, jump reach, HP, damage, TTK, cooldowns, economy, win/loss/restart reset with leak-free cleanup of old loops/timers), preload assets cleanly, engineer airtight collisions from the start (visuals match hitboxes, axis-separated/swept checks, clamped/fixed physics dt <= 0.05s, reliable ground/seam/edge handling, 3D camera collision & pitch clamp so there are zero floor/texture fall-throughs), ALWAYS include a crisp high-contrast crosshair/reticle with shot/hit/reload feedback whenever the game has any weapon or aimed attack — AND when ADS / aiming down sights exists on a weapon with sights/optics, actually bring the weapon model to center and align the camera through the weapon's iron sights / red-dot / scope (NEVER fake ADS with a raw camera FOV zoom while the gun stays off to the side), build complete enemies/entities matched to the genre (whenever enemies fight, equip them with visible weapons/gear properly socketed in their hands, real animations for idle/move/attack/hit/death, clearly visible telegraphed attacks — real muzzle flashes, tracers/projectiles fired from the enemy's gun barrel in shooters, or readable weapon swings/abilities in melee/action games, NEVER empty-handed enemies dealing invisible telepathic damage — real hit reactions and death/destruction animations or ragdoll/shatter with collision/damage disabled immediately so enemies NEVER vanish in 1 frame, plus obstacle avoidance, line-of-sight checks, separation, and validated spawns), enforce clean input hygiene (auto-pause & clear held keys on blur/visibilitychange, FPS-independent mouse, clean resize), pleasant non-harsh audio with Mute, pooled dynamic objects, persistent settings/scores, readable non-obstructive HUD, and performance engineering targeting >= 50 FPS (for standalone medium/large full games — NOT third-party game mods — include real working Graphics/Quality Settings: Low/Medium/High presets, shadow quality, draw distance/LOD/effects density, post-processing/AA, and render scale/DPR cap)"

	gameDevAssetSourcingCompact = "unless the user explicitly asks you to build assets from scratch, BY DEFAULT reuse native engine/project libraries first (Unreal Engine: Content/Vault, Starter Content / Engine Content, Fab / Quixel Megascans, MetaHuman, Game Animation Sample; Unity/Godot: Asset Store Free / AssetLib) and download ready realistic/stylized 3D models, 3D weapons, PBR textures, character animations, 2D sprites, and audio matched by style & category — read prime://skills/prime-gamedev/references/asset-sources.md for curated [Direct / API / No-Login] sources (Poly Haven, AmbientCG, 3D Valley, Grab3D, Polyfork, Gobkit, StylizedTextures.com, Kenney, Quaternius, Mesh2Motion, Game-icons.net, Smithsonian 3D) vs account-gated sources (never waste turns scraping login walls), or search the web freely when needed. Craft from scratch only if requested or no suitable asset exists. Always adapt downloaded models, weapons, textures, and animations to the character/scene (metre scale, weapon hand-grip/pivot, PBR maps, skeleton retargeting, foot-ground contact, root motion, blends/speed), and if ANY asset or animation looks bad or even slightly awkward, edit/fix or replace it yourself"

	gameDevShowcaseRule = "build a clean standalone Showcase scene (e.g. ?showcase=1 or a dedicated inspection view with neutral lighting) placing EVERY created, engine-library, AND downloaded visual element separately (characters, armed enemies, weapons & ADS sight view, items, props, structures, effects, mod additions), run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine dump) photographing every element from multiple sides (front, side, back, 3/4 for 3D; close-up and key states for 2D), AND for full games/levels also capture screenshots of the assembled map/level (overhead/flyover + key gameplay-camera views); when a small or detailed prop/element (e.g. keyboard, weapon sight, desk prop, console panel, seam) is hard to judge from wide views, take a targeted close-up screenshot of that specific element (only when needed — never zoom aimlessly). Inspect ALL photos/contact sheets with ruthless self-critique via view: never accept bare-minimum scenes with just a few props or lazy flat slabs/boxes for small details (such as a flat slab instead of a real keyboard or crude primitives for room props) — every visible object down to small props must look recognizable, properly proportioned, and adequately detailed (download/adapt quality models or model real detail); fix or remake anything that looks rough, sloppy, floating/sunk, gapped, badly lit, or poorly animated and re-shoot until clean"

	gameDevCollisionTestsRule = "write and run deterministic automated collision tests in code (using the project's test runner or presets/collision_suite_probe.mjs) instead of teleporting a bot around the gameplay map: verify every solid object/wall/floor/prop/door/item actually has a registered collider matching its rendered visual bounds, blocks movement from ALL sides (all 4 sides in 2D / all 6 faces in 3D), handles corners and floor/platform seams cleanly, prevents high-speed tunneling (clamped/swept dt <= 0.05s), enforces map bounds and 3D camera wall collision, and passes NaN/Infinity guards"

	gameDevLiveBotRule = "run a real live gameplay bot in the background without hijacking the user's mouse or keyboard — inside Prime Desktop's built-in browser (browser_navigate, browser_record_start/stop, browser_run, browser_press_key with holdMs, browser_evaluate) when browser tools are available, or web_bot_runner.mjs / native_bot_runner.mjs targeting ONLY the game's own window (never capture the full computer screen and never teleport around the map). Slice the recorded video into frames/contact sheets (extract_frames.mjs in a temp directory outside the repo) and check with view + telemetry for controls, menu -> play -> pause -> restart (no double-speed loop leaks), weapon/crosshair firing and true ADS sight alignment (when sights exist), enemy weapons in hand + enemy animations + visible enemy shots/attacks + real death animations (never vanishing in 1 frame), console/WebGL/asset errors, softlocks, sustained FPS >= 50 (optimize if below 50 FPS; for standalone medium/large full games — not mods — confirm Graphics/Quality Settings affect rendering/FPS), and difficulty/balance — do NOT use gameplay video to judge asset appearance (that is Stage A). Bot liveness & anti-hang (MANDATORY): enforce a hard bot timeout (--bot-timeout-ms / bounded loop) and clean teardown, immediately check bot stdout/stderr and visible progress (reject static/frozen screens), and if nothing happens on screen kill the stuck process immediately, fix the bot or input/focus bug, and rerun"
)

func gameDevPromptBlock(agentID string, enabled bool) string {
	return gameDevPromptBlockForSession(agentID, enabled, false, false)
}

func gameDevPromptBlockWithDelegation(agentID string, enabled, allowDelegation bool) string {
	return gameDevPromptBlockForSession(agentID, enabled, allowDelegation, false)
}

func gameDevPromptBlockForSession(agentID string, enabled, allowDelegation, strictVerify bool) string {
	if !enabled {
		return ""
	}
	delegateQAToReviewer := strictVerify && allowDelegation
	switch agentID {
	case config.AgentCode:
		if delegateQAToReviewer {
			return `

<gamedev_mode>
GameDev mode is enabled together with Strict Verify. Follow all GameDev engineering and asset standards upfront, and leave heavy GameDev QA execution to the independent review agent so checks are not duplicated:
First, read prime://skills/prime-gamedev/SKILL.md with view (exposes <skill_directory> and <gamedev_library>).
1. Engineering baseline (whole project for your own game; mod's own elements/UI for a third-party game mod): ` + gameDevEngineeringBaseline + `.
2. Asset sourcing & adaptation: ` + gameDevAssetSourcingCompact + `. Every engine-library, downloaded, and custom asset MUST appear on the Showcase scene.
3. Dedicated Asset Showcase scene (MANDATORY): build a clean standalone showcase/inspection scene (e.g. ?showcase=1 with neutral lighting) placing EVERY created, engine-library, AND downloaded element separately (characters, enemies, weapons, items, props, structures, effects, mod additions) with camera/angle hooks (e.g. window.__SHOWCASE__.listElements() and focusElement(id, angle)) for the review agent's multi-angle photo script.
4. Code-testable collisions (MANDATORY): keep visuals and colliders on a single source of truth so automated code-level collision tests can deterministically verify every solid object blocks movement from all 4/6 sides, corners, seams, tunneling, bounds, and 3D camera walls (no map teleport hacks).
5. Hand QA off to the independent review agent: verify build/launch succeeds, then DO NOT duplicate the full Showcase photo sweep, full collision audit, or live gameplay video recording before review. Fix every defect the reviewer reports and return for fresh re-review.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for ANY game or game mod on any engine, language, or framework).
First, read prime://skills/prime-gamedev/SKILL.md with view (exposes <skill_directory> with scripts/presets/ and <gamedev_library>). Query <gamedev_library> selectively via library.mjs lookup (never read unrelated folders).
1. Engineering baseline (whole project for your own game; mod's own elements/colliders/UI for a third-party game mod): ` + gameDevEngineeringBaseline + `.
2. Asset sourcing & adaptation: ` + gameDevAssetSourcingCompact + `.
3. Stage A — Dedicated Asset Showcase scene & multi-angle photo QA (ALL visuals): ` + gameDevShowcaseRule + `.
4. Stage B — Automated code-level collision tests (NO in-game teleporting): ` + gameDevCollisionTestsRule + `.
5. Stage C — Live gameplay bot & window-scoped video check: ` + gameDevLiveBotRule + `.
6. Fix every bug found across Stages A/B/C and re-verify until clean, save reusable scripts into sorted <gamedev_library> folders (features/, physics/, engines/, elements/, games/) via library.mjs save without deleting existing ones, and clean up temporary video/frame dirs outside the repo.
</gamedev_mode>`
	case config.AgentReview:
		return `

<gamedev_mode>
GameDev mode is enabled. Enforce the MANDATORY three-stage GameDev QA without hijacking the user's mouse or keyboard:
First, read prime://skills/prime-gamedev/SKILL.md with view. Use library.mjs lookup rather than reading unrelated scripts.
1. Fundamentals: confirm balance plan, airtight collisions, dt <= 0.05s, mandatory crosshair/combat feedback when weapons exist, clean HUD/restart, and sustained FPS >= 50 (plus working Graphics/Quality Settings for standalone medium/large full games, NOT mods).
2. Stage A — Showcase multi-angle photo & assembled-map visual audit: open the standalone Showcase scene (all created, engine-library, and downloaded elements including weapons/ADS & animated armed characters), run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs or browser_screenshot / engine capture), AND for full games/levels inspect screenshots of the assembled map/level (overhead/flyover + key gameplay-camera views) with view (taking targeted close-ups of small detailed props like keyboards, sights, or panels ONLY when needed to verify their craft). Apply ruthless quality standards: reject bare-minimum maps with only a few scattered objects or sloppy flat-slab/box shortcuts for small props, plus ANY ugly, misaligned, poorly rigged/animated, floating/sunk, or broken asset, wall/floor gap, lighting hole, or harsh tiling.
3. Stage B — Automated code-level collision test audit (NO in-game teleporting): ` + gameDevCollisionTestsRule + `.
4. Stage C — Live gameplay bot & window-scoped video audit: ` + gameDevLiveBotRule + `.
5. Scope: full game -> verify all elements, world colliders, gameplay, and balance; third-party game mod -> verify only the mod's own elements, colliders, and features.
6. Report any missing QA stage, external-browser usage when built-in browser tools exist, static/frozen bot run, FPS < 50, or unfixed defect so it gets fixed and re-verified cleanly.
</gamedev_mode>`
	case config.AgentPlan:
		return `

<gamedev_mode>
GameDev mode is enabled.
Any implementation and verification plan for a game or mod MUST reference prime://skills/prime-gamedev/SKILL.md (and references/asset-sources.md when sourcing assets) and mandate:
1. Upfront engineering, performance & settings plan: ` + gameDevEngineeringBaseline + ` (skip graphics settings menus for third-party game mods).
2. Selective <gamedev_library> lookup via library.mjs lookup (reusing showcase_capture_probe.mjs, collision_suite_probe.mjs, and saved scripts).
3. Stage A — Asset sourcing & Showcase visual QA: ` + gameDevAssetSourcingCompact + `; ` + gameDevShowcaseRule + `.
4. Stage B — Code-level collision tests (NO in-game teleporting): ` + gameDevCollisionTestsRule + `.
5. Stage C — Live gameplay bot & window-scoped video check (with hard timeout, liveness check, sustained FPS >= 50, and difficulty/balance verification) plus fix-and-rerun loop until clean.
</gamedev_mode>`
	case config.AgentGeneral:
		if delegateQAToReviewer {
			return `

<gamedev_mode>
GameDev mode is enabled together with Strict Verify (MANDATORY for every game or game mod):
- Start by reading prime://skills/prime-gamedev/SKILL.md with view (exposes <skill_directory> and <gamedev_library>).
- Keep roles strictly separated so GameDev QA is NOT duplicated:
  1. Implementation (you or code agents): ` + gameDevEngineeringBaseline + `; ` + gameDevAssetSourcingCompact + `; build the dedicated Showcase scene containing EVERY created, engine-library, and downloaded element separately with camera/angle hooks; keep colliders cleanly testable in code (no map teleport hacks); verify build/launch succeeds. Do NOT duplicate the full Showcase photo sweep, full collision audit, or live gameplay video recording before review.
  2. Independent review agent (MANDATORY): delegate the entire three-stage GameDev QA to the independent review agent — (A) Showcase multi-angle photo audit of all assets/weapons/animations; (B) deterministic code-level collision tests across all 4/6 sides, corners, seams, tunneling, bounds, and 3D camera walls; (C) live gameplay bot + built-in browser / window-scoped video check with hard timeout, anti-freeze liveness check, and FPS >= 50.
  3. Fix every defect reported by the reviewer and send back for fresh re-review until VERDICT: APPROVED, then save reusable scripts to <gamedev_library> without deleting old ones.
</gamedev_mode>`
		}
		if allowDelegation {
			return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod):
- Start by reading prime://skills/prime-gamedev/SKILL.md with view (exposes <skill_directory> and <gamedev_library>).
- Require code agents to follow the engineering baseline (` + gameDevEngineeringBaseline + `), asset sourcing & adaptation (` + gameDevAssetSourcingCompact + `), selective library.mjs lookup, and all three QA stages: (A) Showcase multi-angle visual QA (` + gameDevShowcaseRule + `); (B) code-level collision tests (` + gameDevCollisionTestsRule + `); (C) live gameplay bot & video (` + gameDevLiveBotRule + `), scoped to the whole game for standalone games or only the mod's own features/elements/colliders for third-party game mods.
- Require review agents (or your own verification) to confirm all three stages passed cleanly with zero unfixed bugs and reusable scripts saved to <gamedev_library>.
</gamedev_mode>`
		}
		return `

<gamedev_mode>
GameDev mode is enabled (MANDATORY for every game or game mod):
First read prime://skills/prime-gamedev/SKILL.md with view, query <gamedev_library> selectively via library.mjs lookup, and run the full three-stage loop directly:
1. Engineering baseline (scoped to mod additions for third-party game mods): ` + gameDevEngineeringBaseline + `.
2. Asset sourcing & adaptation: ` + gameDevAssetSourcingCompact + `.
3. Stage A — Showcase multi-angle photo QA: ` + gameDevShowcaseRule + `.
4. Stage B — Code-level collision tests (NO in-game teleporting): ` + gameDevCollisionTestsRule + `.
5. Stage C — Live gameplay bot & window-scoped video check: ` + gameDevLiveBotRule + `.
6. Fix every defect across Stages A/B/C, re-verify until clean, and save reusable scripts to <gamedev_library> via library.mjs save without deleting old ones.
</gamedev_mode>`
	default:
		return ""
	}
}

func hasMCPToolPrefix(tools []fantasy.AgentTool, prefix string) bool {
	for _, tool := range tools {
		if strings.HasPrefix(tool.Info().Name, prefix) {
			return true
		}
	}
	return false
}

func gameDevBlenderBlock(agentID string, env config.BlenderEnvironment, hasBlenderMCP bool) string {
	if !env.Installed && !hasBlenderMCP {
		return ""
	}
	switch agentID {
	case config.AgentGeneral, config.AgentCode, config.AgentPlan:
	default:
		return ""
	}

	var statusLine string
	switch {
	case env.Installed && hasBlenderMCP:
		statusLine = fmt.Sprintf("Blender is installed on this machine (%s) and Blender MCP tools (`mcp_blender_*`) are loaded.", env.ExecutablePath)
	case env.Installed:
		statusLine = fmt.Sprintf("Blender is installed on this machine (%s), while Blender MCP tools are not active yet in this session.", env.ExecutablePath)
	default:
		statusLine = "Blender MCP tools (`mcp_blender_*`) are loaded in this session."
	}

	return `

<blender_3d_workflow>
` + statusLine + `
For 3D games, 3D scenes, or 3D props that need modeling, detailing, or editing (skip for 2D games, non-visual code tasks, or trivial lightweight browser shapes where Blender adds no value):
- If mcp_blender_* tools are loaded, call mcp_blender_get_addon_status first to check whether live Blender is already connected.
- When live Blender IS connected: use **Blender via mcp_blender_* as your primary 3D model editor** — create custom 3D models and properly detailed small props (keyboards, weapons, furniture, room details), import downloaded GLB/FBX/OBJ models into Blender to fix scale, grip pivots, UVs, geometry, and PBR materials, visually inspect them with mcp_blender_look, and export clean GLB/FBX assets into the project.
- When Blender is installed locally but NOT connected yet (or Blender MCP is not active): do NOT block work and do NOT force it — politely **offer once** before 3D modeling to connect Blender, with the short exact steps:
  1. If the addon is not installed yet: run "uvx mcp-for-blender install-addon" (or "mcp-for-blender install-addon") and enable **Edit -> Preferences -> Add-ons -> Interface: MCP for Blender**.
  2. Open Blender, press N in the 3D Viewport -> open the **MCP for Blender** tab -> click **Connect to MCP server** (and make sure the built-in "blender" server is enabled in Settings -> MCP).
  3. Tell the user clearly: once they confirm Blender is connected, you will create and refine 3D models directly in Blender; if they prefer to proceed without Blender, respect that immediately and continue with the normal download/adapt workflow without asking again.
</blender_3d_workflow>`
}
