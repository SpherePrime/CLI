package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func portableSleepCommand(seconds float64) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(
			`powershell -NoProfile -NonInteractive -Command "Start-Sleep -Milliseconds %d"`,
			int(seconds*1000))
	}
	return fmt.Sprintf("sleep %g", seconds)
}

func saveWakeTestIntervals(t *testing.T) {
	t.Helper()
	prevIdle := backgroundWakeIdlePoll
	prevWait := subagentBgWaitPoll
	prevSettle := subagentBgSettleWait
	t.Cleanup(func() {
		backgroundWakeIdlePoll = prevIdle
		subagentBgWaitPoll = prevWait
		subagentBgSettleWait = prevSettle
	})
	backgroundWakeIdlePoll = 15 * time.Millisecond
	subagentBgWaitPoll = 20 * time.Millisecond
	subagentBgSettleWait = 50 * time.Millisecond
}

func startSessionBgShell(t *testing.T, sessionID, cmd string) *shell.BackgroundShell {
	t.Helper()
	bgShell, err := shell.GetBackgroundShellManager().Start(
		context.Background(),
		t.TempDir(),
		nil,
		cmd,
		"test",
		shell.BackgroundOwnership{SessionID: sessionID},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = shell.GetBackgroundShellManager().Kill(bgShell.ID)
	})
	return bgShell
}

func appendUserMsg(t *testing.T, svc message.Service, sessionID, text string) {
	t.Helper()
	_, err := svc.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	})
	require.NoError(t, err)
}

func appendAssistantToolCallMsg(t *testing.T, svc message.Service, sessionID, callID, toolName string) {
	t.Helper()
	_, err := svc.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: callID, Name: toolName, Input: "{}", Finished: true},
		},
	})
	require.NoError(t, err)
}

func appendAssistantTextMsg(t *testing.T, svc message.Service, sessionID, text string) {
	t.Helper()
	_, err := svc.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	})
	require.NoError(t, err)
}

func appendToolResultMsg(t *testing.T, svc message.Service, sessionID, callID, toolName string, metadata any) {
	t.Helper()
	var metaJSON string
	if metadata != nil {
		b, err := json.Marshal(metadata)
		require.NoError(t, err)
		metaJSON = string(b)
	}
	_, err := svc.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: callID,
				Name:       toolName,
				Content:    "ok",
				Metadata:   metaJSON,
			},
		},
	})
	require.NoError(t, err)
}

func TestWakeOnBackgroundJob_HoldsWhileSubagentBusyAndRoutesToJobPending(t *testing.T) {
	saveWakeTestIntervals(t)
	var subBusy atomic.Bool
	subBusy.Store(true)

	mainAgent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(string) bool { return false },
	}
	subAgent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(id string) bool { return id == "sub-session-1" && subBusy.Load() },
	}

	var runnerCalled atomic.Bool
	c := &coordinator{
		mainAgent:     mainAgent,
		mainAgentName: "general",
		subAgents:     map[string]SessionAgent{"code": subAgent},
		agentJobs:     make(map[string]*agentJob),
		backgroundWakeRunner: func(ctx context.Context, sessionID, prompt string) {
			runnerCalled.Store(true)
		},
	}
	job := &agentJob{
		id:        "sub-session-1",
		sessionID: "sub-session-1",
		status:    "running",
		wakeCh:    make(chan struct{}, 1),
		params:    subAgentParams{Agent: subAgent},
	}
	c.agentJobs[job.id] = job

	c.wakeOnBackgroundJob("sub-session-1", "sh-1", "run tests", "go test ./...", 0, "ok")
	time.Sleep(100 * time.Millisecond)

	job.mu.Lock()
	require.Empty(t, job.pending)
	job.mu.Unlock()
	require.False(t, runnerCalled.Load())

	subBusy.Store(false)

	require.Eventually(t, func() bool {
		select {
		case <-job.wakeCh:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)

	job.mu.Lock()
	require.Len(t, job.pending, 1)
	require.Contains(t, job.pending[0], "Background job sh-1 finished with exit code 0")
	job.mu.Unlock()
	require.False(t, runnerCalled.Load())
}

func TestWakeOnMCPJob_HoldsWhileSubagentBusyAndRoutesToJobPending(t *testing.T) {
	saveWakeTestIntervals(t)
	var subBusy atomic.Bool
	subBusy.Store(true)

	subAgent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(id string) bool { return id == "sub-session-mcp" && subBusy.Load() },
	}
	var runnerCalled atomic.Bool
	c := &coordinator{
		mainAgent: &wakeMockAgent{mockSessionAgent: &mockSessionAgent{}, busy: func(string) bool { return false }},
		agentJobs: make(map[string]*agentJob),
		backgroundWakeRunner: func(ctx context.Context, sessionID, prompt string) {
			runnerCalled.Store(true)
		},
	}
	job := &agentJob{
		id:        "sub-session-mcp",
		sessionID: "sub-session-mcp",
		status:    "running",
		wakeCh:    make(chan struct{}, 1),
		params:    subAgentParams{Agent: subAgent},
	}
	c.agentJobs[job.id] = job

	c.wakeOnMCPJob("sub-session-mcp", "mcp-99", "mcp_browser_eval", "done", "result-body")
	time.Sleep(100 * time.Millisecond)

	job.mu.Lock()
	require.Empty(t, job.pending)
	job.mu.Unlock()
	require.False(t, runnerCalled.Load())

	subBusy.Store(false)

	require.Eventually(t, func() bool {
		select {
		case <-job.wakeCh:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)

	job.mu.Lock()
	require.Len(t, job.pending, 1)
	require.Contains(t, job.pending[0], "Background MCP job mcp-99 (mcp_browser_eval) finished with status done")
	job.mu.Unlock()
	require.False(t, runnerCalled.Load())
}

func TestWakeOnBackgroundAndMCPJob_NeverResurrectsFinishedSubagent(t *testing.T) {
	saveWakeTestIntervals(t)

	for _, terminalStatus := range []string{"done", "error", "canceled"} {
		t.Run(terminalStatus, func(t *testing.T) {
			var runnerCalled atomic.Bool
			c := &coordinator{
				mainAgent: &wakeMockAgent{mockSessionAgent: &mockSessionAgent{}, busy: func(string) bool { return false }},
				agentJobs: make(map[string]*agentJob),
				backgroundWakeRunner: func(ctx context.Context, sessionID, prompt string) {
					runnerCalled.Store(true)
				},
			}
			job := &agentJob{
				id:        "sub-finished",
				sessionID: "sub-finished",
				status:    terminalStatus,
				wakeCh:    make(chan struct{}, 1),
			}
			c.agentJobs[job.id] = job

			c.wakeOnBackgroundJob("sub-finished", "sh-dev", "dev server", "npm run dev", 0, "exited")
			c.wakeOnMCPJob("sub-finished", "mcp-1", "mcp_tool", "done", "ok")
			time.Sleep(100 * time.Millisecond)

			job.mu.Lock()
			require.Empty(t, job.pending)
			job.mu.Unlock()
			require.False(t, runnerCalled.Load())
			select {
			case <-job.wakeCh:
				t.Fatal("wakeCh should not be signaled for terminal job")
			default:
			}
		})
	}
}

func TestShouldWaitForSubagentBackgroundJobs(t *testing.T) {
	t.Run("returns false for review agent with final verdict", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		sess, err := env.sessions.Create(t.Context(), "rev")
		require.NoError(t, err)

		bgShell := startSessionBgShell(t, sess.ID, portableSleepCommand(10))
		appendUserMsg(t, env.messages, sess.ID, "review")
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName)
		appendToolResultMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
			Background: true,
			ShellID:    bgShell.ID,
		})
		appendAssistantTextMsg(t, env.messages, sess.ID, "VERDICT: APPROVED")

		job := &agentJob{id: sess.ID, sessionID: sess.ID, agent: config.AgentReview}
		require.False(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "VERDICT: APPROVED\nDEFECTS: none"))
		require.False(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "VERDICT: CHANGES_REQUIRED\n- [major] bug"))
	})

	t.Run("returns true for auto_background shell even after subsequent tool calls", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		sess, err := env.sessions.Create(t.Context(), "code")
		require.NoError(t, err)

		bgShell := startSessionBgShell(t, sess.ID, portableSleepCommand(10))
		appendUserMsg(t, env.messages, sess.ID, "build and test")
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName)
		appendToolResultMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
			Background:     true,
			AutoBackground: true,
			ShellID:        bgShell.ID,
		})
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-2", "view")
		appendToolResultMsg(t, env.messages, sess.ID, "tc-2", "view", nil)
		appendAssistantTextMsg(t, env.messages, sess.ID, "Waiting for background test run.")

		job := &agentJob{id: sess.ID, sessionID: sess.ID, agent: config.AgentTask}
		require.True(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "Waiting for background test run."))
	})

	t.Run("returns true for explicit background shell in final tool batch", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		sess, err := env.sessions.Create(t.Context(), "task")
		require.NoError(t, err)

		bgShell := startSessionBgShell(t, sess.ID, portableSleepCommand(10))
		appendUserMsg(t, env.messages, sess.ID, "run check")
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName)
		appendToolResultMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
			Background: true,
			ShellID:    bgShell.ID,
		})
		appendAssistantTextMsg(t, env.messages, sess.ID, "Waiting for job.")

		job := &agentJob{id: sess.ID, sessionID: sess.ID, agent: config.AgentTask}
		require.True(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "Waiting for job."))
	})

	t.Run("returns false for earlier dev server followed by non-job tools", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		sess, err := env.sessions.Create(t.Context(), "review")
		require.NoError(t, err)

		bgShell := startSessionBgShell(t, sess.ID, portableSleepCommand(10))
		appendUserMsg(t, env.messages, sess.ID, "check app")
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName)
		appendToolResultMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
			Background: true,
			ShellID:    bgShell.ID,
		})
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-2", "view")
		appendToolResultMsg(t, env.messages, sess.ID, "tc-2", "view", nil)
		appendAssistantTextMsg(t, env.messages, sess.ID, "Finished checking.")

		job := &agentJob{id: sess.ID, sessionID: sess.ID, agent: config.AgentReview}
		require.False(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "Finished checking."))
	})

	t.Run("returns false when no running shells exist or result was already seen", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		sess, err := env.sessions.Create(t.Context(), "task")
		require.NoError(t, err)

		bgShell := startSessionBgShell(t, sess.ID, portableSleepCommand(10))
		bgShell.MarkResultSeen()

		appendUserMsg(t, env.messages, sess.ID, "run")
		appendAssistantToolCallMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName)
		appendToolResultMsg(t, env.messages, sess.ID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
			Background:     true,
			AutoBackground: true,
			ShellID:        bgShell.ID,
		})

		job := &agentJob{id: sess.ID, sessionID: sess.ID, agent: config.AgentTask}
		require.False(t, c.shouldWaitForSubagentBackgroundJobs(t.Context(), job, "done"))
	})
}

func TestExecuteWorker_WaitsForActiveBackgroundShellThenResumesSubagent(t *testing.T) {
	saveWakeTestIntervals(t)
	env := testEnv(t)
	c := newJobCoordinator(t, env)

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	var mainRunCalls atomic.Int32
	c.mainAgent = &mockSessionAgent{
		model: Model{
			CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1024},
			ModelCfg:   config.SelectedModel{Provider: "job-provider"},
		},
		runFunc: func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			mainRunCalls.Add(1)
			return agentResultWithText("unexpected main run"), nil
		},
	}

	var mu sync.Mutex
	var prompts []string
	var bgShell *shell.BackgroundShell
	subBusy := &atomic.Bool{}
	firstTurnDone := make(chan struct{})

	subAgent := &wakeMockAgent{
		mockSessionAgent: newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			subBusy.Store(true)
			defer subBusy.Store(false)

			mu.Lock()
			prompts = append(prompts, call.Prompt)
			turn := len(prompts)
			mu.Unlock()

			if turn == 1 {
				appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
				bgShell = startSessionBgShell(t, call.SessionID, portableSleepCommand(10))
				appendAssistantToolCallMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName)
				appendToolResultMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
					Background:     true,
					AutoBackground: true,
					ShellID:        bgShell.ID,
				})
				appendAssistantTextMsg(t, env.messages, call.SessionID, "Waiting for background shell")

				go func(sid string, sh *shell.BackgroundShell) {
					sh.Wait()
					_, _, done, execErr := sh.GetOutput()
					if !done || shell.IsInterrupt(execErr) {
						return
					}
					c.wakeOnBackgroundJob(sid, sh.ID, "bg", "work", 0, "bg-result-ok")
				}(call.SessionID, bgShell)

				close(firstTurnDone)
				return agentResultWithText("Waiting for background shell"), nil
			}

			appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
			appendAssistantTextMsg(t, env.messages, call.SessionID, "VERDICT: APPROVED")
			return agentResultWithText("VERDICT: APPROVED"), nil
		}),
		busy: func(string) bool { return subBusy.Load() },
	}

	job, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          subAgent,
		SessionID:      parent.ID,
		AgentMessageID: "m-bg",
		ToolCallID:     "c-bg",
		Prompt:         "verify project",
		SessionTitle:   "Agent Session: review | rev",
	}, config.AgentReview)
	require.NoError(t, err)

	<-firstTurnDone
	require.Eventually(t, func() bool {
		return bgShell != nil && c.hasActiveSubagentBackgroundJobs(job.sessionID)
	}, 2*time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	statusBefore, _ := job.snapshot()
	require.Equal(t, "running", statusBefore)

	finishedShell := startSessionBgShell(t, job.sessionID, "echo fast-done")
	finishedShell.Wait()
	c.wakeOnBackgroundJob(job.sessionID, finishedShell.ID, "bg", "echo fast-done", 0, "bg-result-ok")
	_ = shell.GetBackgroundShellManager().Kill(bgShell.ID)

	require.True(t, job.wait(t.Context(), 5*time.Second))
	statusAfter, outputAfter := job.snapshot()
	require.Equal(t, "done", statusAfter)
	require.Equal(t, "VERDICT: APPROVED", outputAfter)

	mu.Lock()
	require.Len(t, prompts, 2)
	require.Contains(t, prompts[0], "verify project")
	require.Contains(t, prompts[1], "bg-result-ok")
	mu.Unlock()
	require.Zero(t, mainRunCalls.Load())
}

func TestExecuteWorker_DoesNotDropSleepingWakeUpFinishedJustBeforeTurnEnds(t *testing.T) {
	prevIdle := backgroundWakeIdlePoll
	prevWait := subagentBgWaitPoll
	prevSettle := subagentBgSettleWait
	t.Cleanup(func() {
		backgroundWakeIdlePoll = prevIdle
		subagentBgWaitPoll = prevWait
		subagentBgSettleWait = prevSettle
	})
	backgroundWakeIdlePoll = 250 * time.Millisecond
	subagentBgWaitPoll = 20 * time.Millisecond
	subagentBgSettleWait = 50 * time.Millisecond

	env := testEnv(t)
	c := newJobCoordinator(t, env)

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	var mu sync.Mutex
	var prompts []string
	subBusy := &atomic.Bool{}

	subAgent := &wakeMockAgent{
		mockSessionAgent: newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			subBusy.Store(true)
			defer subBusy.Store(false)

			mu.Lock()
			prompts = append(prompts, call.Prompt)
			turn := len(prompts)
			mu.Unlock()

			if turn == 1 {
				appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
				bgShell := startSessionBgShell(t, call.SessionID, "echo finished-early")
				bgShell.Wait()

				appendAssistantToolCallMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName)
				appendToolResultMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
					Background:     true,
					AutoBackground: true,
					ShellID:        bgShell.ID,
				})
				appendAssistantTextMsg(t, env.messages, call.SessionID, "Waiting for background shell")

				c.wakeOnBackgroundJob(call.SessionID, bgShell.ID, "bg", "echo finished-early", 0, "early-wake-payload")
				time.Sleep(30 * time.Millisecond)
				return agentResultWithText("Waiting for background shell"), nil
			}

			appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
			appendAssistantTextMsg(t, env.messages, call.SessionID, "final-after-early-wake")
			return agentResultWithText("final-after-early-wake"), nil
		}),
		busy: func(string) bool { return subBusy.Load() },
	}

	job, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          subAgent,
		SessionID:      parent.ID,
		AgentMessageID: "m-race",
		ToolCallID:     "c-race",
		Prompt:         "run build",
		SessionTitle:   "Agent Session: task | race",
	}, config.AgentTask)
	require.NoError(t, err)

	require.True(t, job.wait(t.Context(), 5*time.Second))
	statusAfter, outputAfter := job.snapshot()
	require.Equal(t, "done", statusAfter)
	require.Equal(t, "final-after-early-wake", outputAfter)

	mu.Lock()
	require.Len(t, prompts, 2)
	require.Equal(t, "run build", prompts[0])
	require.Contains(t, prompts[1], "early-wake-payload")
	mu.Unlock()
}

func TestExecuteWorker_FinishesWhenBackgroundShellKilledOrJobStopped(t *testing.T) {
	t.Run("shell killed while waiting", func(t *testing.T) {
		saveWakeTestIntervals(t)
		env := testEnv(t)
		c := newJobCoordinator(t, env)

		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		var bgShell *shell.BackgroundShell
		subAgent := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
			bgShell = startSessionBgShell(t, call.SessionID, portableSleepCommand(10))
			appendAssistantToolCallMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName)
			appendToolResultMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
				Background:     true,
				AutoBackground: true,
				ShellID:        bgShell.ID,
			})
			appendAssistantTextMsg(t, env.messages, call.SessionID, "waiting on shell")
			return agentResultWithText("waiting on shell"), nil
		})

		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          subAgent,
			SessionID:      parent.ID,
			AgentMessageID: "m-kill",
			ToolCallID:     "c-kill",
			Prompt:         "work",
		}, config.AgentTask)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return bgShell != nil && c.hasActiveSubagentBackgroundJobs(job.sessionID)
		}, 2*time.Second, 10*time.Millisecond)

		statusBefore, _ := job.snapshot()
		require.Equal(t, "running", statusBefore)

		require.NoError(t, shell.GetBackgroundShellManager().Kill(bgShell.ID))
		require.True(t, job.wait(t.Context(), 5*time.Second))
		statusAfter, outputAfter := job.snapshot()
		require.Equal(t, "done", statusAfter)
		require.Equal(t, "waiting on shell", outputAfter)
	})

	t.Run("job stopped while waiting", func(t *testing.T) {
		saveWakeTestIntervals(t)
		env := testEnv(t)
		c := newJobCoordinator(t, env)

		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		var bgShell *shell.BackgroundShell
		subAgent := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			appendUserMsg(t, env.messages, call.SessionID, call.Prompt)
			bgShell = startSessionBgShell(t, call.SessionID, portableSleepCommand(10))
			appendAssistantToolCallMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName)
			appendToolResultMsg(t, env.messages, call.SessionID, "tc-1", tools.BashToolName, tools.BashResponseMetadata{
				Background:     true,
				AutoBackground: true,
				ShellID:        bgShell.ID,
			})
			appendAssistantTextMsg(t, env.messages, call.SessionID, "waiting on shell")
			return agentResultWithText("waiting on shell"), nil
		})

		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          subAgent,
			SessionID:      parent.ID,
			AgentMessageID: "m-stop",
			ToolCallID:     "c-stop",
			Prompt:         "work",
		}, config.AgentTask)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return bgShell != nil && c.hasActiveSubagentBackgroundJobs(job.sessionID)
		}, 2*time.Second, 10*time.Millisecond)

		require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, job.id))
		require.True(t, job.wait(t.Context(), 5*time.Second))
		statusAfter, _ := job.snapshot()
		require.Equal(t, "canceled", statusAfter)
	})
}
