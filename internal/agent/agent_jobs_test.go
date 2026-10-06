package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// blockingMockAgent is a SessionAgent whose Run blocks until released, so a
// background job can be observed in the "running" state and then killed. The
// fantasy language model is never used here: startAgentJob drives Run, and the
// provider is resolved from ModelCfg.
type blockingMockAgent struct {
	model   Model
	blockOn chan struct{}
}

func (m *blockingMockAgent) Model() Model { return m.model }

func (m *blockingMockAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	select {
	case <-m.blockOn:
		return agentResultWithText("done-after-release"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *blockingMockAgent) BeginAccepted(sessionID string) *AcceptedRun {
	return &AcceptedRun{sessionID: sessionID}
}
func (m *blockingMockAgent) SetModels(large, small Model)                {}
func (m *blockingMockAgent) SetTools(tools []fantasy.AgentTool)          {}
func (m *blockingMockAgent) SetSystemPrompt(s string)                    {}
func (m *blockingMockAgent) Cancel(sessionID string)                     {}
func (m *blockingMockAgent) CancelAll()                                  {}
func (m *blockingMockAgent) IsSessionBusy(sessionID string) bool         { return false }
func (m *blockingMockAgent) IsBusy() bool                                { return false }
func (m *blockingMockAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *blockingMockAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *blockingMockAgent) ClearQueue(sessionID string)                 {}
func (m *blockingMockAgent) Summarize(context.Context, string, fantasy.ProviderOptions, func(context.Context, *fantasy.ProviderError) error) error {
	return nil
}
func (m *blockingMockAgent) GenerateTitle(context.Context, string, string) {}

var _ SessionAgent = (*blockingMockAgent)(nil)

func newBlockingAgent(providerID string) *blockingMockAgent {
	return &blockingMockAgent{
		model: Model{
			CatwalkCfg: catwalk.Model{DefaultMaxTokens: 4096},
			ModelCfg:   config.SelectedModel{Provider: providerID},
		},
		blockOn: make(chan struct{}),
	}
}

// waitForJob polls findJob until the job leaves "running" or the timeout
// elapses, so a test never depends on goroutine scheduling luck.
func waitForJob(t *testing.T, c *coordinator, id string) (status, output string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job := c.findJob(id)
		require.NotNil(t, job, "job %s must be registered", id)
		status, output = job.snapshot()
		if status != "running" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s still running after 5s", id)
	return
}

func agentJobsCall(t *testing.T, c *coordinator, params agentJobsParams) fantasy.ToolResponse {
	t.Helper()
	b, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := c.agentJobsTool().Run(t.Context(), fantasy.ToolCall{Input: string(b)})
	require.NoError(t, err)
	return resp
}

// newJobCoordinator builds a coordinator with a single provider so
// startAgentJob can resolve the model config.
func newJobCoordinator(t *testing.T, env fakeEnv) *coordinator {
	providerID := "job-provider"
	return newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
}

// A background job is registered, runs to completion, and reports its output.
func TestAgentJobRunsToCompletion(t *testing.T) {
	env := testEnv(t)
	coord := newJobCoordinator(t, env)
	providerID := "job-provider"

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	agent := newBlockingAgent(providerID)
	job, err := coord.startAgentJob(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "m-1",
		ToolCallID:     "c-1",
		Prompt:         "go",
		SessionTitle:   "job",
	}, "general")
	require.NoError(t, err)
	require.NotEmpty(t, job.id)

	close(agent.blockOn)
	status, output := waitForJob(t, coord, job.id)
	require.Equal(t, "done", status)
	require.Equal(t, "done-after-release", output)
}

// Starting several jobs concurrently must yield distinct, ordered ids: the
// registry's jobMu serializes the sequence increment even when many
// sub-agents are launched at once.
func TestStartAgentJobConcurrentIDsStayUnique(t *testing.T) {
	const n = 5
	env := testEnv(t)
	coord := newJobCoordinator(t, env)
	providerID := "job-provider"

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		jobs   []*agentJob
		agents []*blockingMockAgent
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			agent := newBlockingAgent(providerID)
			job, err := coord.startAgentJob(t.Context(), subAgentParams{
				Agent:          agent,
				SessionID:      parent.ID,
				AgentMessageID: "m",
				ToolCallID:     fmt.Sprintf("c-%d", seq),
				Prompt:         "go",
				SessionTitle:   "job",
			}, "general")
			require.NoError(t, err)
			mu.Lock()
			jobs = append(jobs, job)
			agents = append(agents, agent)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Release every job in the test goroutine so the detached runs finish
	// instead of leaking.
	for _, a := range agents {
		close(a.blockOn)
	}

	seen := map[string]bool{}
	for _, j := range jobs {
		require.False(t, seen[j.id], "duplicate job id %s", j.id)
		seen[j.id] = true
	}
	require.Len(t, seen, n, "every concurrently started job gets a unique id")

	// The snapshot is ordered by sequence and contains all of them.
	require.Len(t, coord.registrySnapshot(), n)
}

// The agent_jobs tool surfaces the registry state and its error paths.
func TestAgentJobsToolOps(t *testing.T) {
	env := testEnv(t)
	coord := newJobCoordinator(t, env)
	providerID := "job-provider"

	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "list"}).Content, "No background agent jobs")
	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "get", JobID: "aj-999"}).Content, "not found")
	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "kill", JobID: "aj-999"}).Content, "not found")
	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "get"}).Content, "job_id is required")

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	agent := newBlockingAgent(providerID)
	job, err := coord.startAgentJob(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "m",
		ToolCallID:     "c",
		Prompt:         "go",
		SessionTitle:   "job",
	}, "general")
	require.NoError(t, err)

	close(agent.blockOn)
	waitForJob(t, coord, job.id)

	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "list"}).Content, job.id)
	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "get", JobID: job.id}).Content, "done-after-release")
	require.Contains(t, agentJobsCall(t, coord, agentJobsParams{Op: "wait_all"}).Content, job.id)
}

// A still-running job can be killed and reports "canceled".
func TestAgentJobKill(t *testing.T) {
	env := testEnv(t)
	coord := newJobCoordinator(t, env)
	providerID := "job-provider"

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	agent := newBlockingAgent(providerID)
	job, err := coord.startAgentJob(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "m",
		ToolCallID:     "c",
		Prompt:         "go",
		SessionTitle:   "job",
	}, "general")
	require.NoError(t, err)

	coord.killAgentJob(agentJobsParams{JobID: job.id})
	status, _ := waitForJob(t, coord, job.id)
	require.Equal(t, "canceled", status)
}
