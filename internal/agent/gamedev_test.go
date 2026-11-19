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
	require.NotContains(t, lower, "playthrough")
	require.NotContains(t, lower, "test bot")
	require.NotContains(t, lower, "softlock")
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
	require.Contains(t, codeBlock, "playthrough test bots")
	require.Contains(t, codeBlock, "bugs")
	require.Contains(t, codeBlock, "difficulty balance")
	require.Contains(t, codeBlock, "do NOT run tests")

	reviewBlock := gameDevPromptBlock(config.AgentReview, true)
	require.Contains(t, reviewBlock, "<gamedev_mode>")
	require.Contains(t, reviewBlock, "playthrough test bots")
	require.Contains(t, reviewBlock, "bugs")
	require.Contains(t, reviewBlock, "difficulty")
	require.Contains(t, reviewBlock, "defects")

	planBlock := gameDevPromptBlock(config.AgentPlan, true)
	require.Contains(t, planBlock, "<gamedev_mode>")
	require.Contains(t, planBlock, "playthrough test bots")
	require.Contains(t, planBlock, "difficulty balance")

	coordBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, true)
	require.Contains(t, coordBlock, "<gamedev_mode>")
	require.Contains(t, coordBlock, "code agents")
	require.Contains(t, coordBlock, "review agents")
	require.Contains(t, coordBlock, "playthrough test bots")
	require.Contains(t, coordBlock, "difficulty balance")

	directBlock := gameDevPromptBlockWithDelegation(config.AgentGeneral, true, false)
	require.Contains(t, directBlock, "<gamedev_mode>")
	require.Contains(t, directBlock, "write and run automated playthrough test bots")
	require.Contains(t, directBlock, "difficulty balance")
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
		{agentID: config.AgentGeneral, want: "playthrough test bots"},
		{agentID: config.AgentCode, want: "do NOT run tests"},
		{agentID: config.AgentReview, want: "defects"},
		{agentID: config.AgentPlan, want: "difficulty balance"},
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
						require.Contains(t, sys, "difficulty")
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
					require.Contains(t, promptText, "playthrough test bots")
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
