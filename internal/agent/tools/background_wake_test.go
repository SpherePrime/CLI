package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type wakeRecord struct {
	sessionID   string
	shellID     string
	description string
	command     string
	exitCode    int
	output      string
}

func newBashToolWithWake(workingDir string, wake BackgroundJobWake) fantasy.AgentTool {
	permissions := &mockBashPermissionService{}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(permissions, workingDir, workingDir, attribution, "test-model", wake)
}

func waitForWake(t *testing.T, wakes *[]wakeRecord, mu *sync.Mutex) wakeRecord {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		mu.Lock()
		if len(*wakes) > 0 {
			record := (*wakes)[0]
			mu.Unlock()
			return record
		}
		mu.Unlock()
		if time.Now().After(deadline) {
			require.FailNow(t, "wake-up was not delivered")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestBashTool_WakesOnExplicitBackgroundCompletion(t *testing.T) {
	workingDir := t.TempDir()
	var mu sync.Mutex
	var wakes []wakeRecord
	tool := newBashToolWithWake(workingDir, func(sessionID, shellID, description, command string, exitCode int, output string) {
		mu.Lock()
		defer mu.Unlock()
		wakes = append(wakes, wakeRecord{sessionID, shellID, description, command, exitCode, output})
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "wake-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:     "wake check",
		Command:         sleepThenCommand(1.5, "echo wake-marker"),
		RunInBackground: true,
	})
	require.False(t, resp.IsError)

	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.Background)
	require.NotEmpty(t, meta.ShellID)

	record := waitForWake(t, &wakes, &mu)
	require.Equal(t, "wake-session", record.sessionID)
	require.Equal(t, meta.ShellID, record.shellID)
	require.Equal(t, "wake check", record.description)
	require.Equal(t, 0, record.exitCode)
	require.Contains(t, record.output, "wake-marker")
}

func TestBashTool_WakesOnAutoBackgroundCompletion(t *testing.T) {
	workingDir := t.TempDir()
	var mu sync.Mutex
	var wakes []wakeRecord
	tool := newBashToolWithWake(workingDir, func(sessionID, shellID, description, command string, exitCode int, output string) {
		mu.Lock()
		defer mu.Unlock()
		wakes = append(wakes, wakeRecord{sessionID, shellID, description, command, exitCode, output})
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "auto-wake-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:         "auto wake check",
		Command:             sleepThenCommand(1.5, "echo auto-marker"),
		AutoBackgroundAfter: 1,
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "moved to background")

	record := waitForWake(t, &wakes, &mu)
	require.Equal(t, "auto-wake-session", record.sessionID)
	require.Equal(t, 0, record.exitCode)
	require.Contains(t, record.output, "auto-marker")
}

func TestBashTool_DoesNotWakeForKilledJob(t *testing.T) {
	workingDir := t.TempDir()
	var mu sync.Mutex
	var wakes []wakeRecord
	tool := newBashToolWithWake(workingDir, func(sessionID, shellID, description, command string, exitCode int, output string) {
		mu.Lock()
		defer mu.Unlock()
		wakes = append(wakes, wakeRecord{sessionID, shellID, description, command, exitCode, output})
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "kill-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:     "killed job",
		Command:         sleepThenCommand(30, "echo never"),
		RunInBackground: true,
	})
	require.False(t, resp.IsError)
	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.NotEmpty(t, meta.ShellID)

	require.NoError(t, shell.GetBackgroundShellManager().Kill(meta.ShellID))

	time.Sleep(1500 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, wakes)
}

func TestWatchBackgroundJob_IgnoresNilWake(t *testing.T) {
	bgShell, err := shell.GetBackgroundShellManager().Start(
		context.Background(), t.TempDir(), nil, "echo silent", "silent job")
	require.NoError(t, err)
	require.NotPanics(t, func() {
		watchBackgroundJob(bgShell, "session", nil)
	})
}

func TestWatchBackgroundJob_TrimsOutputTail(t *testing.T) {
	bgShell, err := shell.GetBackgroundShellManager().Start(
		context.Background(), t.TempDir(), nil, "echo trailing-spaces   ", "trim job")
	require.NoError(t, err)

	done := make(chan wakeRecord, 1)
	go watchBackgroundJob(bgShell, "trim-session", func(sessionID, shellID, description, command string, exitCode int, output string) {
		done <- wakeRecord{sessionID, shellID, description, command, exitCode, output}
	})

	select {
	case record := <-done:
		require.Equal(t, "trim-session", record.sessionID)
		require.Equal(t, 0, record.exitCode)
		require.True(t, strings.HasSuffix(record.output, "trailing-spaces"))
		require.False(t, strings.HasSuffix(record.output, "   "))
	case <-time.After(15 * time.Second):
		require.FailNow(t, "wake-up was not delivered")
	}
}
