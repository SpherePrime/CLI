package voice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWatchdogStopsIdleServerAndRestartsPrime(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, saveServerState(statePath, ServerState{
		PID: 4242, Port: 8999, LastUsed: time.Now().Add(-time.Hour),
	}))

	killed := 0
	restarted := 0
	err := RunServerWatchdog(context.Background(), ServerWatchdogOptions{
		PID:        4242,
		Port:       8999,
		StatePath:  statePath,
		Idle:       time.Minute,
		CheckEvery: 5 * time.Millisecond,
		Kill:       func(int) error { killed++; return nil },
		Healthy:    func(context.Context, int) bool { return true },
		Restart:    func() error { restarted++; return nil },
	})
	require.NoError(t, err)
	require.Equal(t, 1, killed, "the idle server is stopped once")
	require.Equal(t, 1, restarted, "Prime is brought back detached")
	_, ok := loadServerState(statePath)
	require.False(t, ok, "the record is removed with the server")
}

func TestWatchdogIgnoresReplacedServer(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, saveServerState(statePath, ServerState{
		PID: 99, Port: 8999, LastUsed: time.Now().Add(-time.Hour),
	}))

	killed := 0
	err := RunServerWatchdog(context.Background(), ServerWatchdogOptions{
		PID:        4242,
		Port:       8999,
		StatePath:  statePath,
		Idle:       time.Minute,
		CheckEvery: 5 * time.Millisecond,
		Kill:       func(int) error { killed++; return nil },
		Healthy:    func(context.Context, int) bool { return true },
		Restart:    func() error { return nil },
	})
	require.NoError(t, err)
	require.Zero(t, killed, "a watchdog never stops a server that is not its own")
}

func TestWatchdogWaitsWhileDictationKeepsComing(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, saveServerState(statePath, ServerState{
		PID: 4242, Port: 8999, LastUsed: time.Now(),
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunServerWatchdog(ctx, ServerWatchdogOptions{
			PID:        4242,
			Port:       8999,
			StatePath:  statePath,
			Idle:       time.Hour,
			CheckEvery: 5 * time.Millisecond,
			Kill:       func(int) error { return errors.New("must not kill") },
			Healthy:    func(context.Context, int) bool { return true },
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled, "a busy server keeps the watchdog waiting")
}

func TestWatchdogForgetsDeadServerWithoutKilling(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, saveServerState(statePath, ServerState{
		PID: 4242, Port: 8999, LastUsed: time.Now().Add(-time.Hour),
	}))

	killed := 0
	restarted := 0
	err := RunServerWatchdog(context.Background(), ServerWatchdogOptions{
		PID:        4242,
		Port:       8999,
		StatePath:  statePath,
		Idle:       time.Minute,
		CheckEvery: 5 * time.Millisecond,
		Kill:       func(int) error { killed++; return nil },
		Healthy:    func(context.Context, int) bool { return false },
		Restart:    func() error { restarted++; return nil },
	})
	require.NoError(t, err)
	require.Zero(t, killed, "a port nobody holds must not point a kill at a reused pid")
	require.Equal(t, 1, restarted)
	_, ok := loadServerState(statePath)
	require.False(t, ok)
}

func TestPrimeRunningDetection(t *testing.T) {
	old := scanProcesses
	t.Cleanup(func() { scanProcesses = old })

	// An unreadable process table means "assume alive": the watchdog must
	// never launch a hidden duplicate out of ignorance.
	scanProcesses = func() ([]processEntry, error) { return nil, errors.New("no perms") }
	require.True(t, primeRunning("prime.exe", []string{"--opencode-stdio", "foo"}))

	scanProcesses = func() ([]processEntry, error) {
		return []processEntry{
			{PID: 1, CommandLine: `C:\Windows\System32\svchost.exe -k netsvcs`},
		}, nil
	}
	require.False(t, primeRunning("prime.exe", []string{}))

	scanProcesses = func() ([]processEntry, error) {
		return []processEntry{
			{PID: 1, CommandLine: `C:\tools\prime.exe --opencode-stdio foo`},
		}, nil
	}
	require.True(t, primeRunning("prime.exe", []string{"--opencode-stdio", "foo"}))

	// A development build named after the repo still counts as Prime.
	scanProcesses = func() ([]processEntry, error) {
		return []processEntry{
			{PID: 1, CommandLine: `C:\work\CLI\CLI.exe resume abc`},
		}, nil
	}
	require.True(t, primeRunning(`C:\work\CLI\CLI.exe`, []string{"resume", "abc"}))

	// The watchdog itself carries "prime" in its own command line and must
	// not count as a live session.
	scanProcesses = func() ([]processEntry, error) {
		return []processEntry{{PID: os.Getpid(), CommandLine: `prime voice server watchdog --pid 9`}}, nil
	}
	require.False(t, primeRunning("prime.exe", []string{}))
}

func TestPrimeCommandArgsDropsGlobalFlags(t *testing.T) {
	got := primeCommandArgs([]string{"--data-dir", `C:\dir with spaces`, "--opencode-stdio", "child", "--verbose"})
	require.Equal(t, []string{"child"}, got,
		"global flags are Prime's own choice, and --data-dir swallows its value")
}

func TestTerminalRestartPolicy(t *testing.T) {
	require.Nil(t, terminalRestartArgs([]string{"prime", "voice", "server", "start"}),
		"replaying a tool command would only resurrect the server being stopped")
	require.Nil(t, terminalRestartArgs([]string{"prime", "voice", "test"}))
	require.Nil(t, terminalRestartArgs([]string{"prime", "help"}))

	require.Equal(t, []string{}, terminalRestartArgs([]string{"prime"}),
		"a bare interactive Prime is exactly what the user wants back")
	require.Equal(t, []string{"resume", "abc"}, terminalRestartArgs([]string{"prime", "resume", "abc"}))
	require.Equal(t, []string{"--data-dir", `D:\x`}, terminalRestartArgs([]string{"prime", "--data-dir", `D:\x`}))
}

func TestWatchdogWithoutReplayFlagStopsOnly(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, saveServerState(statePath, ServerState{
		PID: 4242, Port: 8999, LastUsed: time.Now().Add(-time.Hour),
	}))

	killed := 0
	spawned := 0
	useServerTestState(t, 8999)
	spawnDetached = func(serverSpec) (int, error) { spawned++; return 1, nil }
	killProcess = func(int) error { killed++; return nil }

	err := RunServerWatchdog(context.Background(), ServerWatchdogOptions{
		PID:        4242,
		Port:       8999,
		StatePath:  statePath,
		Idle:       time.Minute,
		CheckEvery: 5 * time.Millisecond,
		Healthy:    func(context.Context, int) bool { return true },
	})
	require.NoError(t, err)
	require.Equal(t, 1, killed, "the default kill seam still stops the idle server")
	require.Zero(t, spawned, "without --restart-prime nothing is relaunched")
}

func TestParseCSVTableReadsBothWindowsShapes(t *testing.T) {
	wmic := "Node,ProcessId,CommandLine\r\n" +
		"BOX;1337;\"C:\\Program Files\\prime.exe\" --opencode-stdio child\r\n" +
		"BOX;42;C:\\Windows\\explorer.exe\r\n"
	entries, ok := parseCSVTable(wmic, true)
	require.True(t, ok)
	require.Equal(t, 2, len(entries))
	require.Contains(t, entries[0].CommandLine, "--opencode-stdio")
	require.Equal(t, 1337, entries[0].PID)

	ps := `"ProcessId","CommandLine"` + "\n" +
		`"77","C:\tools\prime.exe --flag=value"` + "\n"
	entries, ok = parseCSVTable(ps, false)
	require.True(t, ok)
	require.Equal(t, 1, len(entries))
	require.Equal(t, 77, entries[0].PID)
	require.Contains(t, entries[0].CommandLine, "--flag=value")

	_, ok = parseCSVTable("nonsense without headers\nfoo\n", false)
	require.False(t, ok)
}

func TestRemoveInterruptedSession(t *testing.T) {
	dir := t.TempDir()

	jsonPath := filepath.Join(dir, "running.json")
	writeFile(t, jsonPath, `{"id":"a","state":"running"}`)
	require.NoError(t, removeInterruptedSession(jsonPath))
	require.NoFileExists(t, jsonPath, "a half-finished turn cannot resume and is dropped")

	donePath := filepath.Join(dir, "done.json")
	writeFile(t, donePath, `{"id":"b","state":"done"}`)
	require.NoError(t, removeInterruptedSession(donePath))
	require.FileExists(t, donePath)

	jsonlPath := filepath.Join(dir, "waiting.jsonl")
	writeFile(t, jsonlPath, "{\"role\":\"user\"}\n{\"state\":\"waiting_for_tools\"}\n")
	require.NoError(t, removeInterruptedSession(jsonlPath))
	require.NoFileExists(t, jsonlPath)

	unknownPath := filepath.Join(dir, "unknown.json")
	writeFile(t, unknownPath, "this is not json at all")
	require.NoError(t, removeInterruptedSession(unknownPath))
	require.FileExists(t, unknownPath, "files Prime cannot read are never touched")

	require.NoError(t, removeInterruptedSession(filepath.Join(dir, "missing.json")))
}

func TestPickPrimeExePrefersExistingFiles(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "prime.exe")
	writeFile(t, real, "binary")

	picked := pickPrimeExe(filepath.Join(dir, "gone.exe"), real)
	require.Equal(t, real, picked)
	require.Equal(t, "", pickPrimeExe("", filepath.Join(dir, "also-gone")))
}

func TestServerStatusInReportsState(t *testing.T) {
	useServerTestState(t, 8001)
	layout := serverTestLayout(t)
	require.NoError(t, saveServerState(layout.serverStatePath(), ServerState{
		PID: 9, Port: 8001, Model: "m", LastUsed: time.Now().Add(-2 * time.Minute),
	}))

	serverHealthy = func(context.Context, int) bool { return true }
	info := ServerStatusIn(context.Background(), layout)
	require.True(t, info.Listening)
	require.True(t, info.HasState)
	require.Equal(t, 9, info.State.PID)
	require.Greater(t, info.IdleFor, time.Minute)
	require.Contains(t, info.LogFile, "server.log")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
