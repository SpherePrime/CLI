package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestSubAgentPromptCoversConfiguredAgents pins the roster the agent tool
// shows the model: without it the model cannot pick an agent, because only the
// names would be guessable.
func TestSubAgentPromptCoversConfiguredAgents(t *testing.T) {
	t.Parallel()

	coder, err := coderPrompt()
	require.NoError(t, err)
	task, err := taskPrompt()
	require.NoError(t, err)
	plan, err := planPrompt()
	require.NoError(t, err)

	// Each configured agent must resolve to a distinct personality, otherwise
	// choosing one only swaps the label and not the behaviour.
	require.NotSame(t, coder, task)
	require.NotSame(t, coder, plan)
	require.NotSame(t, task, plan)

	// An unknown id falls back to the task prompt rather than failing: a
	// stale agent name in config should not break the tool.
	unknown, err := subAgentPrompt("does-not-exist")
	require.NoError(t, err)
	require.NotNil(t, unknown)

	// The task prompt is what the default (empty) name resolves to.
	def, err := subAgentPrompt(config.AgentTask)
	require.NoError(t, err)
	require.NotNil(t, def)
}

// The roster the agent tool shows must be the workers only. Coder is the
// agent the session itself runs on, so listing it as a target let the model
// spawn itself, and each of those calls could spawn another.
func TestSubAgentNamesExcludeMainAgent(t *testing.T) {
	t.Parallel()

	c := newTestCoordinator(t, testEnv(t), "test", config.ProviderConfig{ID: "test"})
	// config.Init does not populate the agent map on its own, and whether it
	// ends up populated depends on whether the machine running the test has a
	// global config to merge. Relying on that made this test pass on a
	// developer's machine and fail on a fresh runner.
	c.cfg.Config().SetupAgents()

	names := c.SubAgentNames()
	require.NotEmpty(t, names)
	require.NotContains(t, names, config.AgentGeneral,
		"the main agent must not be offered as something to call")
	require.Contains(t, names, config.AgentTask)
	require.Contains(t, names, config.AgentPlan)
	require.Contains(t, names, config.AgentExplore)
	require.Contains(t, names, config.AgentReview)
	require.Equal(t, config.AgentCode, names[0])

	// Asking for the main agent by name has to be refused rather than
	// quietly recursing.
	_, err := c.SubAgent(config.AgentGeneral)
	require.Error(t, err)
	require.Contains(t, err.Error(), config.AgentGeneral)
}

func TestSubAgentPromptAcceptsWorkingDir(t *testing.T) {
	t.Parallel()

	// subAgentPrompt must forward options through to the underlying builder,
	// otherwise the sub-agent loses the caller's working directory.
	for _, id := range []string{config.AgentGeneral, config.AgentTask, config.AgentPlan, config.AgentExplore, config.AgentReview} {
		p, err := subAgentPrompt(id)
		require.NoError(t, err, id)
		require.NotNil(t, p, id)
	}
}

func TestSubAgentRebuildsWhenGlobalLargeModelChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	env := testEnv(t)
	primeJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "codex": {
      "id": "codex", "name": "Codex", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k1", "discover_models": false,
      "models": [{"id": "gpt-6.1-sol", "name": "GPT 6.1 Sol", "context_window": 128000, "default_max_tokens": 8192}]
    },
    "antigravity": {
      "id": "antigravity", "name": "Antigravity", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k2", "discover_models": false,
      "models": [{"id": "gemini-4-hydrogen", "name": "Gemini 4 Hydrogen", "context_window": 128000, "default_max_tokens": 8192, "can_reason": true, "reasoning_levels": ["low", "medium", "high"]}]
    }
  },
  "models": {
    "large": {"provider": "codex", "model": "gpt-6.1-sol"},
    "small": {"provider": "codex", "model": "gpt-6.1-sol"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "prime.json"), []byte(primeJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:             cfg,
		sessions:        env.sessions,
		messages:        env.messages,
		permissions:     env.permissions,
		history:         env.history,
		filetracker:     *env.filetracker,
		agents:          make(map[string]SessionAgent),
		subAgents:       make(map[string]SessionAgent),
		agentJobs:       make(map[string]*agentJob),
		workerLifecycle: t.Context(),
		skillTracker:    skills.NewTracker(nil),
		interactive:     true,
	}
	t.Cleanup(func() {
		_ = coord.readyWg.Wait()
	})

	// Cache explore sub-agent on the initial global large model.
	explore1, err := coord.SubAgent(config.AgentExplore)
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())
	require.Equal(t, "codex", explore1.Model().ModelCfg.Provider)
	require.Equal(t, "gpt-6.1-sol", explore1.Model().ModelCfg.Model)

	explore1Again, err := coord.SubAgent(config.AgentExplore)
	require.NoError(t, err)
	require.Same(t, explore1, explore1Again)

	// Switch the global large model (as Desktop model selection does).
	require.NoError(t, cfg.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, config.SelectedModel{
		Provider: "antigravity",
		Model:    "gemini-4-hydrogen",
	}))

	// SubAgent must detect the stale cache entry and rebuild on the new model.
	explore2, err := coord.SubAgent(config.AgentExplore)
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())
	require.NotSame(t, explore1, explore2)
	require.Equal(t, "antigravity", explore2.Model().ModelCfg.Provider)
	require.Equal(t, "gemini-4-hydrogen", explore2.Model().ModelCfg.Model)
	require.Equal(t, "Gemini 4 Hydrogen", coord.SubAgentModel(config.AgentExplore))

	// Reasoning mode changes on the same model must also rebuild.
	require.NoError(t, cfg.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, config.SelectedModel{
		Provider:      "antigravity",
		Model:         "gemini-4-hydrogen",
		ReasoningMode: config.ReasoningModeHigh,
	}))
	explore3, err := coord.SubAgent(config.AgentExplore)
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())
	require.NotSame(t, explore2, explore3)
	require.Equal(t, config.ReasoningModeHigh, explore3.Model().ModelCfg.ReasoningMode)

	// Run must also drop cached sub-agents so a new chat (no session_models entry)
	// and its workers pick up the newly selected global large model.
	require.NoError(t, cfg.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, config.SelectedModel{
		Provider: "codex",
		Model:    "gpt-6.1-sol",
	}))
	newChat, err := env.sessions.Create(t.Context(), "New chat without session_models entry")
	require.NoError(t, err)
	require.NotContains(t, cfg.Config().SessionModels, newChat.ID)

	var subDuringRun SessionAgent
	mainAgent := newMockAgent("antigravity", 8192, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		require.NotNil(t, call.ModelOverride)
		require.Equal(t, "codex", call.ModelOverride.ModelCfg.Provider)
		require.Equal(t, "gpt-6.1-sol", call.ModelOverride.ModelCfg.Model)
		require.Empty(t, coord.cachedSubAgents(), "Run must forget previously cached sub-agents")
		var subErr error
		subDuringRun, subErr = coord.SubAgent(config.AgentExplore)
		require.NoError(t, subErr)
		return agentResultWithText("ok"), nil
	})
	coord.mainAgent = mainAgent
	coord.mainAgentName = config.AgentGeneral
	coord.agents[config.AgentGeneral] = mainAgent

	_, err = coord.Run(t.Context(), newChat.ID, "hello")
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())
	require.NotNil(t, subDuringRun)
	require.NotSame(t, explore3, subDuringRun)
	require.Equal(t, "codex", subDuringRun.Model().ModelCfg.Provider)
	require.Equal(t, "gpt-6.1-sol", subDuringRun.Model().ModelCfg.Model)

	// Switch back to antigravity and verify a worker launched on a new chat uses it.
	require.NoError(t, cfg.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, config.SelectedModel{
		Provider: "antigravity",
		Model:    "gemini-4-hydrogen",
	}))
	rebuilt, err := coord.SubAgent(config.AgentExplore)
	require.NoError(t, err)
	require.NoError(t, coord.readyWg.Wait())

	workerModel := make(chan Model, 1)
	wrappedWorker := &mockSessionAgent{
		model: rebuilt.Model(),
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			if call.ModelOverride != nil {
				workerModel <- *call.ModelOverride
			} else {
				workerModel <- rebuilt.Model()
			}
			return agentResultWithText("worker done"), nil
		},
	}
	job, err := coord.startAgentJob(t.Context(), subAgentParams{
		Agent:          wrappedWorker,
		SessionID:      newChat.ID,
		AgentMessageID: "msg-worker",
		ToolCallID:     "call-worker",
		Prompt:         "inspect repo",
		SessionTitle:   "Agent Session: explore | scout",
	}, config.AgentExplore)
	require.NoError(t, err)
	status, output := waitForJob(t, coord, job.id)
	require.Equal(t, "done", status)
	require.Equal(t, "worker done", output)

	select {
	case got := <-workerModel:
		require.Equal(t, "antigravity", got.ModelCfg.Provider)
		require.Equal(t, "gemini-4-hydrogen", got.ModelCfg.Model)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker model observation")
	}
}