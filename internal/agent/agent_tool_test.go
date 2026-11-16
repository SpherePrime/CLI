package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAgentToolMetadataDistinguishesPreLaunchFromLaunchedErrors(t *testing.T) {
	env := testEnv(t)
	coord := newJobCoordinator(t, env)
	coord.cfg.Config().SetupAgents()
	coord.subAgents = make(map[string]SessionAgent)
	coord.workerLifecycle = t.Context()
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	agentTool, err := coord.agentTool(t.Context())
	require.NoError(t, err)

	baseCtx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	baseCtx = context.WithValue(baseCtx, tools.MessageIDContextKey, "msg-1")

	t.Run("pre-launch rejection has no job_id metadata", func(t *testing.T) {
		resp, runErr := agentTool.Run(baseCtx, fantasy.ToolCall{
			ID:    "call-unknown",
			Name:  AgentToolName,
			Input: `{"agent":"does-not-exist","prompt":"work"}`,
		})
		require.NoError(t, runErr)
		require.True(t, resp.IsError)
		require.Empty(t, resp.Metadata)
	})

	t.Run("launched worker error attaches job_id metadata", func(t *testing.T) {
		coord.subAgentMu.Lock()
		coord.subAgents[config.AgentCode] = newMockAgent("job-provider", 1024, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
			return nil, errors.New("worker failed")
		})
		coord.subAgentMu.Unlock()

		resp, runErr := agentTool.Run(baseCtx, fantasy.ToolCall{
			ID:    "call-failed",
			Name:  AgentToolName,
			Input: `{"agent":"code","prompt":"work"}`,
		})
		require.NoError(t, runErr)
		require.True(t, resp.IsError)
		require.NotEmpty(t, resp.Metadata)

		var meta backgroundAgentMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.NotEmpty(t, meta.JobID)
		require.False(t, meta.Background)
	})

	t.Run("canceled foreground wait attaches job_id metadata", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(baseCtx)
		defer cancel()

		coord.subAgentMu.Lock()
		coord.subAgents[config.AgentCode] = newMockAgent("job-provider", 1024, func(runCtx context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			cancel()
			<-runCtx.Done()
			return nil, runCtx.Err()
		})
		coord.subAgentMu.Unlock()

		resp, runErr := agentTool.Run(cancelCtx, fantasy.ToolCall{
			ID:    "call-canceled",
			Name:  AgentToolName,
			Input: `{"agent":"code","prompt":"work"}`,
		})
		require.NoError(t, runErr)
		require.True(t, resp.IsError)
		require.NotEmpty(t, resp.Metadata)

		var meta backgroundAgentMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.NotEmpty(t, meta.JobID)
	})
}
