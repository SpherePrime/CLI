package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func assertNoGameDevMentions(t *testing.T, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	require.NotContains(t, lower, "gamedev")
	require.NotContains(t, lower, "game_dev")
	require.NotContains(t, lower, "prime-gamedev")
	require.NotContains(t, lower, "playthrough")
	require.NotContains(t, lower, "test bot")
	require.NotContains(t, lower, "softlock")
	require.NotContains(t, lower, "extract_frames")
	require.NotContains(t, lower, "web_bot_runner")
	require.NotContains(t, lower, "native_bot_runner")
	require.NotContains(t, lower, "element_capture")
	require.NotContains(t, lower, "gamedev_library")
	require.NotContains(t, lower, "browser_record_start")
	require.NotContains(t, lower, "browser_record_stop")
}

func TestGameDevPromptBlockDisabledReturnsEmpty(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		config.AgentGeneral,
		config.AgentCode,
		config.AgentReview,
		config.AgentPlan,
		config.AgentTask,
		config.AgentExplore,
		config.AgentDesigner,
		config.LegacyAgentCoder,
		"",
	} {
		require.Empty(t, gameDevPromptBlock(id, false))
		require.Empty(t, gameDevPromptBlockWithDelegation(id, false, true))
		require.Empty(t, gameDevPromptBlockWithDelegation(id, false, false))
	}
}

func TestGameDevPromptBlockEnabledByRole(t *testing.T) {
	t.Parallel()

	require.Empty(t, gameDevPromptBlock(config.AgentDesigner, true))
	require.Empty(t, gameDevPromptBlock(config.AgentExplore, true))
	require.Empty(t, gameDevPromptBlock(config.AgentTask, true))

	codeBlock := gameDevPromptBlock(config.AgentCode, true)
	require.Contains(t, codeBlock, "<gamedev_mode>")
	require.Contains(t, codeBlock, "MANDATORY")
	require.Contains(t, codeBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, codeBlock, "<gamedev_library>")
	require.Contains(t, codeBlock, "concrete balance plan before coding")
	require.Contains(t, codeBlock, "airtight collisions")
	require.Contains(t, codeBlock, "crosshair/reticle")
	require.Contains(t, codeBlock, "scripts/presets/")
	require.Contains(t, codeBlock, "library.mjs lookup")
	require.Contains(t, codeBlock, "Dedicated Asset Showcase scene")
	require.Contains(t, codeBlock, "presets/showcase_capture_probe.mjs")
	require.Contains(t, codeBlock, "EVERY created, engine-library, AND downloaded visual element separately")
	require.Contains(t, codeBlock, "Automated code-level collision tests (NO in-game teleporting)")
	require.Contains(t, codeBlock, "presets/collision_suite_probe.mjs")
	require.Contains(t, codeBlock, "all 4 sides in 2D / all 6 faces in 3D")
	require.Contains(t, codeBlock, "features/, physics/")
	require.Contains(t, codeBlock, "without hijacking the user's mouse or keyboard")
	require.Contains(t, codeBlock, "Prime Desktop's built-in browser")
	require.Contains(t, codeBlock, "browser_record_start")
	require.Contains(t, codeBlock, "targeting ONLY the game's own window")
	require.Contains(t, codeBlock, "never capture the full computer screen")
	require.Contains(t, codeBlock, "hard bot timeout")
	require.Contains(t, codeBlock, "static/frozen")
	require.Contains(t, codeBlock, "FPS >= 50")
	require.Contains(t, codeBlock, "shadow quality")
	require.Contains(t, codeBlock, "align the camera through the weapon's iron sights")
	require.Contains(t, codeBlock, "visible weapons/gear properly socketed in their hands")
	require.Contains(t, codeBlock, "tracers/projectiles fired from the enemy's gun barrel")
	require.Contains(t, codeBlock, "references/game-craft.md")
	require.Contains(t, codeBlock, "assembled map/level")
	require.Contains(t, codeBlock, "ruthless self-critique")
	require.Contains(t, codeBlock, "targeted close-up screenshot")
	require.Contains(t, codeBlock, "flat slab instead of a real keyboard")
	require.Contains(t, codeBlock, "NEVER vanish in 1 frame")
	require.Contains(t, codeBlock, "Scale to Scope")
	require.Contains(t, codeBlock, "third-party game mod")
	require.Contains(t, codeBlock, "native_bot_runner.mjs")
	require.Contains(t, codeBlock, "floor/texture fall-throughs")
	require.Contains(t, codeBlock, "Unreal Engine")
	require.Contains(t, codeBlock, "Fab / Quixel Megascans")
	require.Contains(t, codeBlock, "Starter Content / Engine Content")
	require.Contains(t, codeBlock, "BY DEFAULT")
	require.Contains(t, codeBlock, "prime://skills/prime-gamedev/references/asset-sources.md")
	require.Contains(t, codeBlock, "[Direct / API / No-Login]")
	require.Contains(t, codeBlock, "Mesh2Motion")
	require.Contains(t, codeBlock, "Smithsonian 3D")
	require.Contains(t, codeBlock, "skeleton retargeting")
	require.Contains(t, codeBlock, "even slightly awkward, edit/fix or replace it yourself")
	require.Contains(t, codeBlock, "StylizedTextures.com")
	require.Contains(t, codeBlock, "Grab3D")
	require.Contains(t, codeBlock, "Polyfork")
	require.Contains(t, codeBlock, "Gobkit")
	require.Contains(t, codeBlock, "Quaternius")
	require.Contains(t, codeBlock, "Game-icons.net")
	require.Contains(t, codeBlock, "never waste turns scraping login walls")
	require.Contains(t, codeBlock, "Craft from scratch only if requested or no suitable asset exists")
	require.Contains(t, codeBlock, "contact sheets")
	require.Contains(t, codeBlock, "Fix every bug found")
	require.Contains(t, strings.ToLower(codeBlock), "difficulty/balance")
	require.NotContains(t, codeBlock, "do NOT run tests")
	assertNoStrictVerifyMentions(t, codeBlock)

	reviewBlock := gameDevPromptBlock(config.AgentReview, true)
	require.Contains(t, reviewBlock, "<gamedev_mode>")
	require.Contains(t, reviewBlock, "MANDATORY")
	require.Contains(t, reviewBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, reviewBlock, "library.mjs lookup")
	require.Contains(t, reviewBlock, "balance plan")
	require.Contains(t, reviewBlock, "mandatory crosshair")
	require.Contains(t, reviewBlock, "Showcase multi-angle photo & assembled-map visual audit")
	require.Contains(t, reviewBlock, "presets/showcase_capture_probe.mjs")
	require.Contains(t, reviewBlock, "Automated code-level collision test audit (NO in-game teleporting)")
	require.Contains(t, reviewBlock, "presets/collision_suite_probe.mjs")
	require.Contains(t, reviewBlock, "all 4 sides in 2D / all 6 faces in 3D")
	require.Contains(t, reviewBlock, "do NOT use gameplay video to judge asset appearance")
	require.Contains(t, reviewBlock, "without hijacking the user's mouse or keyboard")
	require.Contains(t, reviewBlock, "Prime Desktop's built-in browser")
	require.Contains(t, reviewBlock, "never capture the full computer screen")
	require.Contains(t, reviewBlock, "third-party game mod")
	require.Contains(t, reviewBlock, "contact sheets")
	require.Contains(t, reviewBlock, "difficulty")
	require.Contains(t, reviewBlock, "defect")
	assertNoStrictVerifyMentions(t, reviewBlock)

	planBlock := gameDevPromptBlock(config.AgentPlan, true)
	require.Contains(t, planBlock, "<gamedev_mode>")
	require.Contains(t, planBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, planBlock, "concrete balance plan before coding")
	require.Contains(t, planBlock, "high-contrast crosshair")
	require.Contains(t, planBlock, "Showcase scene")
	require.Contains(t, planBlock, "showcase_capture_probe.mjs")
	require.Contains(t, planBlock, "collision_suite_probe.mjs")
	require.Contains(t, planBlock, "Code-level collision tests (NO in-game teleporting)")
	require.Contains(t, planBlock, "third-party game mods")
	require.Contains(t, planBlock, "Fab / Quixel Megascans")
	require.Contains(t, planBlock, "references/asset-sources.md")
	require.Contains(t, planBlock, "Graphics/Quality Settings")
	require.Contains(t, planBlock, "skip graphics settings menus for third-party game mods")
	require.Contains(t, planBlock, ">= 50 FPS")
	require.Contains(t, planBlock, "Mesh2Motion")
	require.Contains(t, planBlock, "Craft from scratch only if requested or no suitable asset exists")
	require.Contains(t, planBlock, "difficulty/balance")
	require.Contains(t, planBlock, "fix-and-rerun loop")
	assertNoStrictVerifyMentions(t, planBlock)

	coordBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, true)
	require.Contains(t, coordBlock, "<gamedev_mode>")
	require.Contains(t, coordBlock, "MANDATORY")
	require.Contains(t, coordBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, coordBlock, "code agents")
	require.Contains(t, coordBlock, "concrete balance plan")
	require.Contains(t, coordBlock, "high-contrast crosshair")
	require.Contains(t, coordBlock, "Showcase scene")
	require.Contains(t, coordBlock, "EVERY created, engine-library, AND downloaded visual element separately")
	require.Contains(t, coordBlock, "automated collision tests in code")
	require.Contains(t, coordBlock, "instead of teleporting a bot around the gameplay map")
	require.Contains(t, coordBlock, "without hijacking the user's mouse or keyboard")
	require.Contains(t, coordBlock, "Prime Desktop's built-in browser")
	require.Contains(t, coordBlock, "review agents")
	require.Contains(t, coordBlock, "difficulty")
	assertNoStrictVerifyMentions(t, coordBlock)

	directBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, false)
	require.Contains(t, directBlock, "<gamedev_mode>")
	require.Contains(t, directBlock, "MANDATORY")
	require.Contains(t, directBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, directBlock, "draft a concrete balance plan before coding")
	require.Contains(t, directBlock, "high-contrast crosshair")
	require.Contains(t, directBlock, "Showcase scene")
	require.Contains(t, directBlock, "presets/showcase_capture_probe.mjs")
	require.Contains(t, directBlock, "Code-level collision tests (NO in-game teleporting)")
	require.Contains(t, directBlock, "presets/collision_suite_probe.mjs")
	require.Contains(t, directBlock, "without hijacking the user's mouse or keyboard")
	require.Contains(t, directBlock, "Prime Desktop's built-in browser")
	require.Contains(t, directBlock, "never capture the full computer screen")
	require.Contains(t, directBlock, "contact sheets")
	require.Contains(t, directBlock, "Fab / Quixel Megascans")
	require.Contains(t, directBlock, "BY DEFAULT")
	require.Contains(t, directBlock, "Mesh2Motion")
	require.Contains(t, directBlock, "references/asset-sources.md")
	require.Contains(t, directBlock, "Craft from scratch only if requested or no suitable asset exists")
	require.Contains(t, directBlock, "difficulty/balance")
	require.Contains(t, directBlock, "Fix every defect")
	assertNoStrictVerifyMentions(t, directBlock)

	combinedGeneral := gameDevPromptBlockForSession(config.AgentGeneral, true, true, true)
	require.Contains(t, combinedGeneral, "Keep roles strictly separated so GameDev QA is NOT duplicated")
	require.Contains(t, combinedGeneral, "Do NOT duplicate the full Showcase photo sweep, full collision audit, or live gameplay video recording before review")
	require.Contains(t, combinedGeneral, "Independent review agent (MANDATORY)")
	require.Contains(t, combinedGeneral, "deterministic code-level collision tests")
	require.Contains(t, combinedGeneral, "no map teleport hacks")

	combinedCode := gameDevPromptBlockForSession(config.AgentCode, true, true, true)
	require.Contains(t, combinedCode, "Dedicated Asset Showcase scene (MANDATORY)")
	require.Contains(t, combinedCode, "Code-testable collisions (MANDATORY)")
	require.Contains(t, combinedCode, "no map teleport hacks")
	require.Contains(t, combinedCode, "DO NOT duplicate the full Showcase photo sweep, full collision audit, or live gameplay video recording before review")
	require.Contains(t, combinedCode, "Hand QA off to the independent review agent")

	combinedNoDelegation := gameDevPromptBlockForSession(config.AgentGeneral, true, false, true)
	require.Equal(t, directBlock, combinedNoDelegation)
}

func TestSessionAgentPromptZeroMentionsWhenGameDevOff(t *testing.T) {
	for _, agentID := range []string{
		config.AgentGeneral,
		config.AgentCode,
		config.AgentReview,
		config.AgentPlan,
	} {
		t.Run(agentID, func(t *testing.T) {
			env := testEnv(t)
			model := &scriptedOrchestratorModel{
				provider: "mock",
				modelID:  "mock-model",
				steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
					func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
						sys := systemPromptTextInCall(call)
						assertNoGameDevMentions(t, sys)
						return textStopParts("ok", fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}), nil
					},
				},
			}
			sa := NewSessionAgent(SessionAgentOptions{
				LargeModel: Model{
					Model:      model,
					CatwalkCfg: defaultTestCatwalkModel("mock-model"),
					ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
				},
				SmallModel: Model{
					Model:      model,
					CatwalkCfg: defaultTestCatwalkModel("mock-model"),
					ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
				},
				SystemPrompt: "base system prompt",
				IsSubAgent:   agentID != config.AgentGeneral,
				Agent:        config.Agent{ID: agentID},
				Sessions:     env.sessions,
				Messages:     env.messages,
			})
			sess, err := env.sessions.Create(t.Context(), "off-"+agentID)
			require.NoError(t, err)

			_, err = sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID,
				Prompt:    "build something",
				GameDev:   false,
			})
			require.NoError(t, err)
			require.Equal(t, 1, model.callCount())
		})
	}
}

func TestSessionAgentPromptInjectsGameDevWhenOn(t *testing.T) {
	for _, tc := range []struct {
		agentID string
		want    string
	}{
		{agentID: config.AgentGeneral, want: "prime://skills/prime-gamedev/SKILL.md"},
		{agentID: config.AgentCode, want: "contact sheets"},
		{agentID: config.AgentReview, want: "defect"},
		{agentID: config.AgentPlan, want: "difficulty/balance"},
	} {
		t.Run(tc.agentID, func(t *testing.T) {
			env := testEnv(t)
			model := &scriptedOrchestratorModel{
				provider: "mock",
				modelID:  "mock-model",
				steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
					func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
						sys := systemPromptTextInCall(call)
						require.Contains(t, sys, "<gamedev_mode>")
						require.Contains(t, sys, tc.want)
						require.Contains(t, strings.ToLower(sys), "difficulty")
						return textStopParts("ok", fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}), nil
					},
				},
			}
			sa := NewSessionAgent(SessionAgentOptions{
				LargeModel: Model{
					Model:      model,
					CatwalkCfg: defaultTestCatwalkModel("mock-model"),
					ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
				},
				SmallModel: Model{
					Model:      model,
					CatwalkCfg: defaultTestCatwalkModel("mock-model"),
					ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
				},
				SystemPrompt: "base system prompt",
				IsSubAgent:   tc.agentID != config.AgentGeneral,
				Agent:        config.Agent{ID: tc.agentID},
				Sessions:     env.sessions,
				Messages:     env.messages,
			})
			sess, err := env.sessions.Create(t.Context(), "on-"+tc.agentID)
			require.NoError(t, err)

			_, err = sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID,
				Prompt:    "build a game",
				GameDev:   true,
			})
			require.NoError(t, err)
			require.Equal(t, 1, model.callCount())
		})
	}
}

func TestOrchestratorPromptsRespectGameDevToggle(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		name := "off"
		if enabled {
			name = "on"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt := newOrchestratorRuntime(OrchestratorCallConfig{
				Enabled:            true,
				FinalReviewEnabled: true,
				GameDev:            enabled,
				Planner: OrchestratorRoleConfig{
					Model: Model{CatwalkCfg: catwalk.Model{ID: "planner"}},
				},
				Executor: OrchestratorRoleConfig{
					Model: Model{CatwalkCfg: catwalk.Model{ID: "executor"}},
				},
			})

			planPrompt := rt.phaseSystemPrompt()
			rt.phase = OrchestratorPhaseExecute
			execPrompt := rt.phaseSystemPrompt()
			rt.phase = OrchestratorPhaseReview
			reviewPrompt := rt.phaseSystemPrompt()

			for _, promptText := range []string{planPrompt, execPrompt, reviewPrompt} {
				if !enabled {
					assertNoGameDevMentions(t, promptText)
				} else {
					require.Contains(t, promptText, "<gamedev_mode>")
					require.Contains(t, promptText, "prime://skills/prime-gamedev/SKILL.md")
					require.Contains(t, promptText, "Showcase")
					require.Contains(t, promptText, "collisions")
					require.Contains(t, promptText, "difficulty")
				}
			}
		})
	}
}

func TestGameDevBlenderBlock(t *testing.T) {
	noBlender := config.BlenderEnvironment{}
	require.Empty(t, gameDevBlenderBlock(config.AgentGeneral, noBlender, false))
	require.Empty(t, gameDevBlenderBlock(config.AgentCode, noBlender, false))

	installed := config.BlenderEnvironment{
		Installed:      true,
		ExecutablePath: `C:\Program Files\Blender Foundation\Blender 4.3\blender.exe`,
	}
	require.Empty(t, gameDevBlenderBlock(config.AgentReview, installed, true))
	require.Empty(t, gameDevBlenderBlock(config.AgentExplore, installed, true))

	offerBlock := gameDevBlenderBlock(config.AgentGeneral, installed, false)
	require.Contains(t, offerBlock, "<blender_3d_workflow>")
	require.Contains(t, offerBlock, "Blender is installed on this machine")
	require.Contains(t, offerBlock, "offer once")
	require.Contains(t, offerBlock, "uvx mcp-for-blender install-addon")
	require.Contains(t, offerBlock, "Connect to MCP server")
	require.Contains(t, offerBlock, "primary 3D model editor")

	connectedBlock := gameDevBlenderBlock(config.AgentCode, installed, true)
	require.Contains(t, connectedBlock, "Blender MCP tools (`mcp_blender_*`) are loaded")
	require.Contains(t, connectedBlock, "mcp_blender_get_addon_status")
	require.Contains(t, connectedBlock, "mcp_blender_look")
}

func TestCoordinatorGameDevContextAndSubAgentPropagation(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.Config().Options.GameDev = false

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	require.False(t, c.resolveRunGameDev(t.Context(), parent.ID))
	require.False(t, c.isGameDevEnabled(t.Context(), parent.ID))

	overrideCtx := WithGameDev(t.Context(), true)
	require.True(t, c.resolveRunGameDev(overrideCtx, parent.ID))
	require.True(t, c.isGameDevEnabled(t.Context(), parent.ID))

	calls := make(chan SessionAgentCall, 1)
	worker := newMockAgent("job-provider", 1024, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		calls <- call
		return agentResultWithText("done"), nil
	})
	job, err := c.startAgentJob(overrideCtx, subAgentParams{
		Agent:          worker,
		SessionID:      parent.ID,
		AgentMessageID: "m1",
		ToolCallID:     "tc1",
		Prompt:         "work",
		GameDev:        c.isGameDevEnabled(overrideCtx, parent.ID),
	}, config.AgentCode)
	require.NoError(t, err)
	call := <-calls
	require.True(t, call.GameDev)
	require.True(t, job.wait(t.Context(), 5000000000))

	require.False(t, c.resolveRunGameDev(t.Context(), parent.ID))
	require.False(t, c.isGameDevEnabled(t.Context(), parent.ID))
}

func TestCoderPromptExcludesHiddenBuiltinSkill(t *testing.T) {
	t.Parallel()

	out := renderCoderPrompt(t, false)
	assertNoGameDevMentions(t, out)

	smartOut := renderCoderPrompt(t, true)
	assertNoGameDevMentions(t, smartOut)
}

