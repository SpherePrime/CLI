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
	require.Contains(t, codeBlock, "library.mjs lookup")
	require.Contains(t, codeBlock, "actually enters the running game")
	require.Contains(t, codeBlock, "target the game's own window or viewport")
	require.Contains(t, codeBlock, "never capture the full computer screen")
	require.Contains(t, codeBlock, "never substitute static screenshots for video recording")
	require.Contains(t, codeBlock, "element_capture.mjs")
	require.Contains(t, codeBlock, "native_bot_runner.mjs")
	require.Contains(t, codeBlock, "falling through floors/textures")
	require.Contains(t, codeBlock, "contact sheets")
	require.Contains(t, codeBlock, "Fix every bug you find and re-verify")
	require.Contains(t, strings.ToLower(codeBlock), "difficulty/balance")
	require.NotContains(t, codeBlock, "do NOT run tests")

	reviewBlock := gameDevPromptBlock(config.AgentReview, true)
	require.Contains(t, reviewBlock, "<gamedev_mode>")
	require.Contains(t, reviewBlock, "MANDATORY")
	require.Contains(t, reviewBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, reviewBlock, "library.mjs lookup")
	require.Contains(t, reviewBlock, "actually enters the running game")
	require.Contains(t, reviewBlock, "strictly from the target game window or game viewport")
	require.Contains(t, reviewBlock, "NOT the full computer screen")
	require.Contains(t, reviewBlock, "NOT just standalone screenshots")
	require.Contains(t, reviewBlock, "close-up screenshots of individual created elements")
	require.Contains(t, reviewBlock, "dense frames")
	require.Contains(t, reviewBlock, "inspect the contact sheets")
	require.Contains(t, reviewBlock, "difficulty")
	require.Contains(t, reviewBlock, "defect")

	planBlock := gameDevPromptBlock(config.AgentPlan, true)
	require.Contains(t, planBlock, "<gamedev_mode>")
	require.Contains(t, planBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, planBlock, "ONLY the target game window or viewport")
	require.Contains(t, planBlock, "never the full computer screen")
	require.Contains(t, planBlock, "dense frames")
	require.Contains(t, planBlock, "difficulty and balance")
	require.Contains(t, planBlock, "fix-and-rerun loop")

	coordBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, true)
	require.Contains(t, coordBlock, "<gamedev_mode>")
	require.Contains(t, coordBlock, "MANDATORY")
	require.Contains(t, coordBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, coordBlock, "code agents")
	require.Contains(t, coordBlock, "review agents")
	require.Contains(t, coordBlock, "ONLY the target game window/viewport")
	require.Contains(t, coordBlock, "never the full computer screen")
	require.Contains(t, coordBlock, "dense frames")
	require.Contains(t, coordBlock, "difficulty")
	require.Contains(t, coordBlock, "FIX every bug found")

	directBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, false)
	require.Contains(t, directBlock, "<gamedev_mode>")
	require.Contains(t, directBlock, "MANDATORY")
	require.Contains(t, directBlock, "prime://skills/prime-gamedev/SKILL.md")
	require.Contains(t, directBlock, "Write and run a live automated playthrough bot")
	require.Contains(t, directBlock, "ONLY the target game window or viewport")
	require.Contains(t, directBlock, "never capture the full computer screen")
	require.Contains(t, directBlock, "never substitute static screenshots for video")
	require.Contains(t, directBlock, "dense frames")
	require.Contains(t, directBlock, "difficulty and balance")
	require.Contains(t, directBlock, "Fix every bug found")
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
		{agentID: config.AgentPlan, want: "difficulty and balance"},
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
					require.Contains(t, promptText, "playthrough")
					require.Contains(t, promptText, "difficulty")
				}
			}
		})
	}
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

