package agent

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// wakeMockAgent lets a test control IsSessionBusy while reusing mockSessionAgent.
type wakeMockAgent struct {
	*mockSessionAgent
	busy func(string) bool
}

func (m *wakeMockAgent) IsSessionBusy(sessionID string) bool {
	return m.busy(sessionID)
}

func TestBackgroundJobWakePrompt_CarriesTerminalState(t *testing.T) {
	prompt := backgroundJobWakePrompt("042", "run tests", "go test ./...", 1, "FAIL package\n--- FAIL: TestX")

	require.Contains(t, prompt, "Background job 042 finished with exit code 1")
	require.Contains(t, prompt, "run tests")
	require.Contains(t, prompt, "go test ./...")
	require.Contains(t, prompt, "FAIL package")
	require.Contains(t, prompt, "Continue the interrupted work")
}

func TestBackgroundJobWakePrompt_TruncatesLongOutput(t *testing.T) {
	output := strings.Repeat("a", backgroundWakeOutputLimit+500)
	prompt := backgroundJobWakePrompt("007", "", "", 0, output)

	require.False(t, strings.Contains(prompt, output))
	require.True(t, strings.Contains(prompt, "… "))
}

func TestBackgroundJobWakePrompt_ShortOutputKeptWhole(t *testing.T) {
	prompt := backgroundJobWakePrompt("001", "", "", 0, "done")

	require.Contains(t, prompt, "\ndone\n")
	require.NotContains(t, prompt, "… ")
}

func TestWakeOnBackgroundJob_HoldsUntilTurnEnds(t *testing.T) {
	backgroundWakeIdlePoll = 20 * time.Millisecond
	var busy atomic.Bool
	busy.Store(true)
	var mu sync.Mutex
	var runs []string
	agent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(string) bool { return busy.Load() },
	}
	coordinator := &coordinator{mainAgent: agent, mainAgentName: "test"}
	coordinator.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, prompt)
	}

	coordinator.wakeOnBackgroundJob("session-1", "hold-042", "build", "go build", 0, "ok")
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	require.Empty(t, runs)
	mu.Unlock()

	busy.Store(false)
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		count := len(runs)
		mu.Unlock()
		if count == 1 {
			return
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "wake-up was not delivered after the turn ended")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestWakeOnBackgroundJob_SkipsResultAlreadySeen(t *testing.T) {
	bgShell, err := shell.GetBackgroundShellManager().Start(context.Background(), t.TempDir(), nil, "echo seen", "seen")
	require.NoError(t, err)
	t.Cleanup(func() { shell.GetBackgroundShellManager().Remove(bgShell.ID) })
	bgShell.Wait()
	bgShell.MarkResultSeen()

	agent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(string) bool { return false },
	}
	coordinator := &coordinator{mainAgent: agent, mainAgentName: "test"}
	var mu sync.Mutex
	var runs []string
	coordinator.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, prompt)
	}

	coordinator.wakeOnBackgroundJob("session-1", bgShell.ID, "", "", 0, "")
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, runs)
}

func TestWakeOnBackgroundJob_StartsRunForIdleSession(t *testing.T) {
	var mu sync.Mutex
	var runs []struct {
		sessionID string
		prompt    string
		hidden    bool
	}
	agent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(string) bool { return false },
	}
	coordinator := &coordinator{mainAgent: agent, mainAgentName: "test"}
	coordinator.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, struct {
			sessionID string
			prompt    string
			hidden    bool
		}{sessionID, prompt, message.HiddenUserMessage(ctx)})
	}

	coordinator.wakeOnBackgroundJob("session-1", "042", "build", "go build", 0, "ok")

	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		count := len(runs)
		var run struct {
			sessionID string
			prompt    string
			hidden    bool
		}
		if count > 0 {
			run = runs[0]
		}
		mu.Unlock()
		if count > 0 {
			require.Equal(t, "session-1", run.sessionID)
			require.True(t, run.hidden)
			require.Contains(t, run.prompt, "Background job 042 finished with exit code 0")
			return
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "wake-up was not delivered to the idle session")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestWakeOnBackgroundJob_IgnoresEmptySession(t *testing.T) {
	agent := &wakeMockAgent{
		mockSessionAgent: &mockSessionAgent{},
		busy:             func(string) bool { return false },
	}
	coordinator := &coordinator{mainAgent: agent, mainAgentName: "test"}
	coordinator.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
		require.FailNow(t, "no run may be started without a session")
	}

	coordinator.wakeOnBackgroundJob("", "042", "", "", 0, "")
	time.Sleep(200 * time.Millisecond)
}

func TestBackgroundJobWakePrompt_HiddenMessageMarker(t *testing.T) {
	ctx := message.WithHiddenUserMessage(context.Background())
	require.True(t, message.HiddenUserMessage(ctx))
	require.IsType(t, fantasy.AgentResult{}, fantasy.AgentResult{})
}
