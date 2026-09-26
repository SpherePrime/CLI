package voice

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// managedServerPort is overridden per test through this helper; lifecycle
// tests must not run in parallel because they share package seams.
func useServerTestState(t *testing.T, port int) *[]serverSpec {
	t.Helper()

	oldPort := managedServerPort
	oldIdle := ServerIdleTimeout
	oldHealthy := serverHealthy
	oldSpawn := spawnDetached
	oldKill := killProcess
	oldScan := scanProcesses

	managedServerPort = port
	ServerIdleTimeout = time.Hour
	spawned := []serverSpec{}
	spawnDetached = func(spec serverSpec) (int, error) {
		spawned = append(spawned, spec)
		return 4242, nil
	}
	killProcess = func(int) error { return nil }
	scanProcesses = func() ([]processEntry, error) { return nil, errors.New("unused") }

	t.Cleanup(func() {
		managedServerPort = oldPort
		ServerIdleTimeout = oldIdle
		serverHealthy = oldHealthy
		spawnDetached = oldSpawn
		killProcess = oldKill
		scanProcesses = oldScan
	})
	return &spawned
}

// healthServer answers the health route the way whisper-server does once its
// model is loaded.
func healthServer(t *testing.T, ready bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != healthRoute {
			http.NotFound(w, r)
			return
		}
		if !ready {
			http.Error(w, "loading", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("OK"))
	}))
	t.Cleanup(server.Close)
	return server
}

// healthPort pulls the loopback port out of an httptest URL.
func healthPort(t *testing.T, server *httptest.Server) int {
	t.Helper()
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	require.NoError(t, err)
	return port
}

// serverTestLayout is an install tree with a fake whisper-server binary and a
// model file, which is all ensureServer insists on before launching.
func serverTestLayout(t *testing.T) InstallLayout {
	t.Helper()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "bin"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "models"), 0o755)
	exeName := whisperServerBinary
	if filepath.Separator == '\\' {
		exeName += ".exe"
	}
	_ = os.WriteFile(filepath.Join(root, "bin", exeName), []byte("fake"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "models", "ggml-base.bin"), []byte("weights"), 0o600)
	return InstallLayout{Root: root}
}

func TestServerStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")

	_, ok := loadServerState(path)
	require.False(t, ok, "no file means no state")

	state := ServerState{PID: 7, Port: 8000, Model: "m", LastUsed: time.Now().Add(-time.Hour)}
	require.NoError(t, saveServerState(path, state))

	loaded, ok := loadServerState(path)
	require.True(t, ok)
	require.Equal(t, state.PID, loaded.PID)

	touchServerState(path)
	touched, ok := loadServerState(path)
	require.True(t, ok)
	require.True(t, touched.LastUsed.After(loaded.LastUsed), "touching restarts the idle countdown")
	require.Equal(t, state.PID, touched.PID, "touching keeps the record otherwise intact")

	clearServerState(path)
	_, ok = loadServerState(path)
	require.False(t, ok)
}

func TestServerStateRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
	_, ok := loadServerState(path)
	require.False(t, ok)

	require.NoError(t, os.WriteFile(path, []byte(`{"pid":0}`), 0o600))
	_, ok = loadServerState(path)
	require.False(t, ok, "a record without a pid cannot name a server")
}

func TestServerArgs(t *testing.T) {
	args := serverArgs("/models/ggml-base.bin", 8123, "")
	require.Equal(t, "/models/ggml-base.bin", valueAfter(args, "-m"))
	require.Equal(t, "127.0.0.1", valueAfter(args, "--host"), "the server must stay on loopback")
	require.Equal(t, "8123", valueAfter(args, "--port"))
	require.Equal(t, "auto", valueAfter(args, "-l"), "unset language still means detection")

	pinned := serverArgs("/models/ggml-base.bin", 8123, "ru")
	require.Equal(t, "ru", valueAfter(pinned, "-l"), "a configured language skips the per-request detection pass")

	threads, err := strconv.Atoi(valueAfter(args, "-t"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, threads, 4, "the server must not crawl on four workers by default")
	require.LessOrEqual(t, threads, 16)
}

func TestEnsureServerStartsOnceAndSpawnsWatchdog(t *testing.T) {
	backend := healthServer(t, true)
	spawned := useServerTestState(t, healthPort(t, backend))

	// Nothing is running until the spawn fake "succeeds".
	serverHealthy = func(context.Context, int) bool { return len(*spawned) > 0 }

	layout := serverTestLayout(t)
	p := newProber(layout)

	endpoint, err := ensureServer(context.Background(), Settings{Enabled: true}, p)
	require.NoError(t, err)
	require.Equal(t, serverInferenceURL(managedServerPort), endpoint)
	require.Len(t, *spawned, 2, "the server and its idle watchdog are both launched")
	require.Contains(t, filepath.Base((*spawned)[0].Exe), whisperServerBinary)
	require.Equal(t, valueAfter((*spawned)[0].Args, "-m"), filepath.Join(layout.Models(), "ggml-base.bin"))

	watchdog := (*spawned)[1]
	require.Equal(t, []string{"voice", "server", "watchdog"}, watchdog.Args[:3])
	require.Contains(t, strings.Join(watchdog.Args, " "), "--pid 4242")

	state, ok := loadServerState(layout.serverStatePath())
	require.True(t, ok)
	require.Equal(t, 4242, state.PID)
	require.Contains(t, state.Model, "ggml-base.bin")

	// A second call finds the healthy server and launches nothing more.
	before := len(*spawned)
	_, err = ensureServer(context.Background(), Settings{Enabled: true}, p)
	require.NoError(t, err)
	require.Len(t, *spawned, before)
}

func TestEnsureServerRestartsWhenModelChanged(t *testing.T) {
	backend := healthServer(t, true)
	spawned := useServerTestState(t, healthPort(t, backend))

	layout := serverTestLayout(t)
	stale := filepath.Join(layout.Models(), "ggml-old.bin")
	require.NoError(t, os.WriteFile(stale, []byte("weights"), 0o600))
	require.NoError(t, saveServerState(layout.serverStatePath(), ServerState{
		PID: 55, Port: managedServerPort, Model: stale, LastUsed: time.Now(),
	}))

	killed := 0
	killProcess = func(int) error { killed++; return nil }
	// The old server answers until it is stopped; then the replacement does.
	serverHealthy = func(context.Context, int) bool { return killed == 0 || len(*spawned) > 0 }

	_, err := ensureServer(context.Background(), Settings{Enabled: true}, newProber(layout))
	require.NoError(t, err)
	require.Equal(t, 1, killed, "the server holding the wrong model is stopped")
	require.GreaterOrEqual(t, len(*spawned), 1, "the replacement server starts")

	fresh, ok := loadServerState(layout.serverStatePath())
	require.True(t, ok)
	require.NotEqual(t, stale, fresh.Model)
}

func TestEnsureServerRestartsWhenLanguagePinned(t *testing.T) {
	backend := healthServer(t, true)
	spawned := useServerTestState(t, healthPort(t, backend))

	layout := serverTestLayout(t)
	require.NoError(t, saveServerState(layout.serverStatePath(), ServerState{
		PID: 55, Port: managedServerPort, Model: filepath.Join(layout.Models(), "ggml-base.bin"),
		LastUsed: time.Now(),
	}))

	killed := 0
	killProcess = func(int) error { killed++; return nil }
	serverHealthy = func(context.Context, int) bool { return killed == 0 || len(*spawned) > 0 }

	// The user pinned Russian after the server started with auto-detect.
	_, err := ensureServer(context.Background(), Settings{Enabled: true, Language: "ru"}, newProber(layout))
	require.NoError(t, err)
	require.Equal(t, 1, killed, "a pinned language restarts the detecting server")

	state, ok := loadServerState(layout.serverStatePath())
	require.True(t, ok)
	require.Equal(t, "ru", state.Language)
	require.Equal(t, "ru", valueAfter((*spawned)[0].Args, "-l"))
}

func TestEnsureServerNeedsBinaryAndModel(t *testing.T) {
	useServerTestState(t, 8123)
	serverHealthy = func(context.Context, int) bool { return false }

	_, err := ensureServer(context.Background(), Settings{}, newProber(InstallLayout{Root: t.TempDir()}))
	require.Error(t, err)
	require.Contains(t, err.Error(), whisperServerBinary)

	layout := serverTestLayout(t)
	require.NoError(t, os.RemoveAll(layout.Models()))
	_, err = ensureServer(context.Background(), Settings{}, newProber(layout))
	require.Error(t, err)
	require.Contains(t, err.Error(), "model")
}

func TestWarmTranscriberFallsBackToCommand(t *testing.T) {
	useServerTestState(t, 8123)
	serverHealthy = func(context.Context, int) bool { return false }
	spawnDetached = func(serverSpec) (int, error) { return 0, errors.New("no spawn in tests") }

	layout := serverTestLayout(t)
	failing := &recordingTranscriber{err: errors.New("whisper.cpp died")}
	engine := warmServerTranscriber{cli: failing, settings: Settings{}, p: newProber(layout)}
	require.Equal(t, "whisper.cpp server (warming)", engine.Name(),
		"a dead server is admitted in the label before it is started")

	recording := &Audio{WAV: []byte("wav bytes"), Duration: time.Second}
	_, err := engine.Transcribe(context.Background(), recording)
	require.Error(t, err, "both the server problem and the command failure are reported")
	require.Contains(t, err.Error(), "no spawn in tests")
	require.Contains(t, err.Error(), "whisper.cpp died")

	// A CLI that answers still wins while the server is down.
	helped := &recordingTranscriber{text: "hello"}
	engine.cli = helped
	text, err := engine.Transcribe(context.Background(), recording)
	require.NoError(t, err)
	require.Equal(t, "hello", text)
	require.Equal(t, 1, helped.calls)
}

// recordingTranscriber is a fake engine that counts uses.
type recordingTranscriber struct {
	text  string
	err   error
	calls int
}

func (r *recordingTranscriber) Name() string { return "fake" }

func (r *recordingTranscriber) Transcribe(context.Context, *Audio) (string, error) {
	r.calls++
	return r.text, r.err
}

func TestKeepModelWarmNeedsServerBinary(t *testing.T) {
	cli := &recordingTranscriber{}

	plain := keepModelWarm(cli, Settings{}, newProber(InstallLayout{Root: t.TempDir()}))
	require.Equal(t, Transcriber(cli), plain, "without the server binary the command is returned untouched")

	layout := serverTestLayout(t)
	wrapped := keepModelWarm(cli, Settings{}, newProber(layout))
	_, ok := wrapped.(warmServerTranscriber)
	require.True(t, ok, "an installed whisper-server upgrades the command engine")
}

func TestStopServerInClearsState(t *testing.T) {
	useServerTestState(t, 8001)
	layout := serverTestLayout(t)
	require.NoError(t, saveServerState(layout.serverStatePath(), ServerState{PID: 9, Port: 8001}))

	stopped, err := StopServerIn(context.Background(), layout)
	require.NoError(t, err)
	require.False(t, stopped, "a dead server is only forgotten, not killed")

	require.NoError(t, saveServerState(layout.serverStatePath(), ServerState{PID: 9, Port: 8001}))
	serverHealthy = func(context.Context, int) bool { return true }
	killed := 0
	killProcess = func(int) error { killed++; return nil }

	stopped, err = StopServerIn(context.Background(), layout)
	require.NoError(t, err)
	require.True(t, stopped)
	require.Equal(t, 1, killed)
	_, ok := loadServerState(layout.serverStatePath())
	require.False(t, ok, "stopping forgets the server so a watchdog stops chasing it")
}

func TestTouchServerIn(t *testing.T) {
	useServerTestState(t, 8001)
	layout := serverTestLayout(t)
	require.False(t, TouchServerIn(layout), "nothing recorded means nothing to touch")

	require.NoError(t, saveServerState(layout.serverStatePath(),
		ServerState{PID: 9, Port: 8001, LastUsed: time.Now().Add(-time.Hour)}))
	require.True(t, TouchServerIn(layout))

	state, _ := loadServerState(layout.serverStatePath())
	require.Less(t, time.Since(state.LastUsed), time.Minute, "the idle countdown restarted")
}
