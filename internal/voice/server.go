package voice

// Prime keeps a whisper.cpp HTTP server resident while the voice plugin is
// enabled. A one-shot whisper-cli run reloads a half-gigabyte model from
// disk on every keypress, which costs tens of seconds; the server loads it
// once and answers transcriptions in about a second. The pieces here start
// the server when first needed and record it on disk so the plugin
// deactivate hook can stop it later.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	// whisperServerBinary is the HTTP server whisper.cpp ships beside
	// whisper-cli, so the archive Prime already downloads includes it.
	whisperServerBinary = "whisper-server"

	// healthRoute is whisper.cpp server's readiness endpoint, which starts
	// answering only once the model is fully loaded.
	healthRoute = "/health"

	// serverStartPoll is how often ensureServer retries the health route
	// while the model loads.
	serverStartPoll = 250 * time.Millisecond

	// serverStopWait bounds how long to wait for a restarted server's port
	// to free up after the old process is killed.
	serverStopWait = 5 * time.Second
)

var (
	// managedServerPort is where Prime's own whisper server listens. It is
	// the port auto-detection already probes, so a warm server becomes the
	// preferred engine on its own.
	managedServerPort = 8000

	// serverStartTimeout bounds waiting for a large model to load.
	serverStartTimeout = 3 * time.Minute

	// ensureServerMu serialises server creation inside one Prime process so
	// concurrent callers share one server.
	ensureServerMu sync.Mutex
)

// ServerState is Prime's on-disk record of the managed whisper server.
type ServerState struct {
	PID      int       `json:"pid"`
	Port     int       `json:"port"`
	Model    string    `json:"model"`
	Language string    `json:"language"`
	Exe      string    `json:"exe"`
	StartedAt time.Time `json:"started_at"`
}

// serverStatePath and serverLogPath keep the managed server's bookkeeping
// inside Prime's own voice directory.
func (l InstallLayout) serverStatePath() string {
	return filepath.Join(l.Root, "server.json")
}

func (l InstallLayout) serverLogPath() string {
	return filepath.Join(l.Root, "server.log")
}

// loadServerState reads the record written by saveServerState.
func loadServerState(path string) (ServerState, bool) {
	if path == "" {
		return ServerState{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ServerState{}, false
	}
	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil || state.PID <= 0 {
		return ServerState{}, false
	}
	return state, true
}

// saveServerState persists the record with a private mode, then returns.
func saveServerState(path string, state ServerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// clearServerState forgets the managed server.
func clearServerState(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// touchServerState rewrites the state file so the record reflects the server
// is still alive; it does not stop the server on its own.
func touchServerState(path string) {
	state, ok := loadServerState(path)
	if !ok {
		return
	}
	_ = saveServerState(path, state)
}

// serverHealthURL is where the managed server reports readiness.
func serverHealthURL(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + healthRoute
}

// serverInferenceURL is the transcription route of the managed server.
func serverInferenceURL(port int) string {
	return inferenceEndpoint(serverHealthURL(port))
}

// ServerEndpoint is where the managed whisper server accepts transcriptions.
func ServerEndpoint() string {
	return serverInferenceURL(managedServerPort)
}

// TouchServer records that the managed server was just used, restarting its
// idle countdown without starting anything.
func TouchServer() bool {
	return TouchServerIn(DefaultLayout())
}

// TouchServerIn records a use of the server held by one install layout.
func TouchServerIn(layout InstallLayout) bool {
	path := layout.serverStatePath()
	if _, ok := loadServerState(path); !ok {
		return false
	}
	touchServerState(path)
	return true
}

// serverHealthy asks the health route directly. Any answer below 500 means a
// whisper server is listening with its model loaded; while a server is still
// loading, the port refuses connections outright.
var serverHealthy = func(ctx context.Context, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, engineProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverHealthURL(port), nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode < http.StatusInternalServerError
}

// serverArgs builds the whisper-server command line: loopback only, a fixed
// port, every core the machine has, and the configured language so the server
// does not burn a 30-second detection pass on each short dictation.
func serverArgs(model string, port int, language string) []string {
	return []string{
		"-m", model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"-t", strconv.Itoa(serverThreads()),
		"-l", orDefault(language, "auto"),
	}
}

// serverThreads lets the resident model use the whole machine. whisper.cpp
// defaults to four workers, which roughly triples dictation latency on
// modern CPUs; logical cores are capped because the extra SMT siblings add
// little to ggml workloads.
func serverThreads() int {
	threads := runtime.NumCPU()
	if threads > 16 {
		threads = 16
	}
	return threads
}

// serverSpec describes one detached process Prime starts and leaves running.
type serverSpec struct {
	Exe  string
	Args []string
	Env  []string
	// Log receives the process output; nil discards it.
	Log *os.File
	// NewConsole asks for a fresh visible terminal instead of a hidden
	// detached process, which is how a restarted Prime window comes back.
	NewConsole bool
}

// spawnDetached starts spec outside Prime's process group and returns its
// pid. It is a variable so tests can watch what would be launched.
var spawnDetached = defaultSpawnDetached

// killProcess stops a process Prime started. Tests swap it out.
var killProcess = defaultKillProcess

// ensureServer makes Prime's managed whisper server answer and returns the
// inference endpoint, starting (or restarting) the server when needed.
func ensureServer(ctx context.Context, settings Settings, p prober) (string, error) {
	ensureServerMu.Lock()
	defer ensureServerMu.Unlock()

	statePath := p.layout.serverStatePath()
	endpoint := serverInferenceURL(managedServerPort)

	if serverHealthy(ctx, managedServerPort) {
		state, hasState := loadServerState(statePath)
		model, modelOK := serverModelFile(settings, p)
		if !hasState || (modelOK && state.Model == model && state.Language == settings.Language) {
			touchServerState(statePath)
			return endpoint, nil
		}
		// The model or language changed since the server started, so the
		// resident process would transcribe with the wrong weights or spend
		// every request re-detecting a language the user already pinned.
		if err := killProcess(state.PID); err == nil {
			waitForServerDown(ctx, managedServerPort)
			time.Sleep(serverKillGrace)
		}
		clearServerState(statePath)
	}

	exe, ok := p.layout.Binary(whisperServerBinary)
	if !ok {
		return "", fmt.Errorf("%s is not installed", whisperServerBinary)
	}
	model, ok := serverModelFile(settings, p)
	if !ok {
		return "", errors.New("no ggml model found for the whisper server")
	}
	// Health failed, so whatever the state file remembered is gone; drop it
	// rather than killing a pid that may since have been reused.
	clearServerState(statePath)

	logFile, err := os.OpenFile(p.layout.serverLogPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("cannot open the whisper server log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	pid, err := spawnDetached(serverSpec{
		Exe:  exe,
		Args: serverArgs(model, managedServerPort, settings.Language),
		Env:  p.layout.libraryPathEnv(),
		Log:  logFile,
	})
	if err != nil {
		return "", fmt.Errorf("cannot start the whisper server: %w", err)
	}

	now := time.Now()
	state := ServerState{
		PID:       pid,
		Port:      managedServerPort,
		Model:     model,
		Language:  settings.Language,
		Exe:       exe,
		StartedAt: now,
	}
	if err := saveServerState(statePath, state); err != nil {
		return "", fmt.Errorf("cannot record the whisper server: %w", err)
	}

	return waitForServerUp(ctx, managedServerPort, endpoint, statePath)
}

// waitForServerUp polls the health route until the model finishes loading.
func waitForServerUp(ctx context.Context, port int, endpoint, statePath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, serverStartTimeout)
	defer cancel()
	for {
		if serverHealthy(ctx, port) {
			return endpoint, nil
		}
		select {
		case <-ctx.Done():
			clearServerState(statePath)
			return "", fmt.Errorf("%w: the whisper server did not become ready; see %s",
				ErrTranscribeFailed, DefaultLayout().serverLogPath())
		case <-time.After(serverStartPoll):
		}
	}
}

// waitForServerDown waits for a killed server to release its port.
func waitForServerDown(ctx context.Context, port int) {
	ctx, cancel := context.WithTimeout(ctx, serverStopWait)
	defer cancel()
	for serverHealthy(ctx, port) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(serverStartPoll):
		}
	}
}

// serverModelFile resolves the model the managed server should load, which is
// the configured one or whatever voice setup downloaded.
func serverModelFile(settings Settings, p prober) (string, bool) {
	return whisperModelFile(settings, p.layout)
}

// warmServerTranscriber keeps the Whisper model resident: it ensures the
// managed server is running and posts audio there, falling back to the
// one-shot whisper.cpp command whenever the server cannot be used.
type warmServerTranscriber struct {
	cli      Transcriber
	settings Settings
	p        prober
}

func (t warmServerTranscriber) Name() string {
	if serverHealthy(context.Background(), managedServerPort) {
		return "whisper.cpp server"
	}
	// The label tells reports and the test command that the first use will
	// load the model instead of answering from a running server.
	return "whisper.cpp server (warming)"
}

func (t warmServerTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	endpoint, err := ensureServer(ctx, t.settings, t.p)
	if err == nil {
		server := newServerTranscriber(endpoint, t.settings)
		text, serverErr := server.Transcribe(ctx, audio)
		if serverErr == nil {
			return text, nil
		}
		err = serverErr
	}
	text, cliErr := t.cli.Transcribe(ctx, audio)
	if cliErr == nil {
		return text, nil
	}
	return "", errors.Join(err, cliErr)
}

// keepModelWarm wraps a working whisper.cpp command with the managed server
// when the server binary Prime installed is available.
func keepModelWarm(cli Transcriber, settings Settings, p prober) Transcriber {
	if _, ok := p.layout.Binary(whisperServerBinary); !ok {
		return cli
	}
	return warmServerTranscriber{cli: cli, settings: settings, p: p}
}

// WarmServer starts the managed Whisper server so the model is in RAM before
// the first dictation. Calling it repeatedly is cheap, and it is a no-op on
// machines where the server cannot be the engine anyway.
func WarmServer(ctx context.Context, settings Settings) error {
	if !settings.Enabled || settings.TranscribeCommand != "" {
		return nil
	}
	switch settings.Engine {
	case "", EngineAuto, EngineServer:
	default:
		return nil
	}
	p := newProber(DefaultLayout())
	if _, ok := p.layout.Binary(whisperServerBinary); !ok {
		return nil
	}
	_, err := ensureServer(ctx, settings, p)
	return err
}

// ServerInfo describes the managed server as Prime can see it.
type ServerInfo struct {
	State     ServerState
	HasState  bool
	Listening bool
	Port      int
	IdleFor   time.Duration
	LogFile   string
}

// ServerStatus reports whether the managed server is answering and, when Prime
// started one, which model it holds and how long it has been idle.
func ServerStatus(ctx context.Context) ServerInfo {
	return ServerStatusIn(ctx, DefaultLayout())
}

// ServerStatusIn answers the same questions for one install layout.
func ServerStatusIn(ctx context.Context, layout InstallLayout) ServerInfo {
	info := ServerInfo{Port: managedServerPort, LogFile: layout.serverLogPath()}
	info.Listening = serverHealthy(ctx, managedServerPort)
	info.State, info.HasState = loadServerState(layout.serverStatePath())
	return info
}

// StopServer shuts the managed Whisper server down, reporting whether one was
// live. A server Prime did not start is left alone.
func StopServer(ctx context.Context) (bool, error) {
	return StopServerIn(ctx, DefaultLayout())
}

// StopServerIn stops the server recorded in one install layout. The return
// says whether a live process was stopped; a record of an already-dead
// server is only forgotten.
func StopServerIn(ctx context.Context, layout InstallLayout) (bool, error) {
	state, ok := loadServerState(layout.serverStatePath())
	if !ok {
		return false, nil
	}
	wasLive := serverHealthy(ctx, state.Port)
	if wasLive {
		if err := killProcess(state.PID); err != nil {
			return false, fmt.Errorf("could not stop the whisper server (pid %d): %w", state.PID, err)
		}
	}
	clearServerState(layout.serverStatePath())
	return wasLive, nil
}
