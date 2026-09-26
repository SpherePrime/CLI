// Package voice runs Prime's microphone dictation as a stdio MCP server.
// Agents get dictate/setup/status/warm/stop_server tools; the heavy lifting
// stays in internal/voice, which keeps one resident Whisper server per
// machine so transcriptions answer in seconds instead of reloading the model
// on every call.
package voice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/version"
	voiceengine "github.com/SpherePrime/CLI/internal/voice"
	mcp "github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
)

// dictateDefaultMaxSeconds bounds a recording unless the caller asks for a
// different length.
const dictateDefaultMaxSeconds = 30

type service struct {
	settings voiceengine.Settings

	mu       sync.Mutex
	detector *voiceengine.Detector
	warming  bool
}

// Serve connects the voice MCP server to stdin/stdout until ctx is done.
func Serve(ctx context.Context) error {
	s, err := newService()
	if err != nil {
		return err
	}

	server := s.mcpServer()

	go s.warmNow()

	slog.Info("prime-voice MCP server started", "engine", s.settings.Engine, "model", s.settings.Model)
	return server.Run(ctx, &mcp.StdioTransport{})
}

// mcpServer wires the tools once so tests can reach the same server over an
// in-memory transport.
func (s *service) mcpServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "prime-voice",
		Version: version.Version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "dictate",
		Description: "Record the microphone and return the dictated text. Recording ends by itself " +
			"when the speaker goes quiet (or at max_seconds, default 30), so call it, wait, and use " +
			"the result. Tell the user to speak before calling. Errors explain what is missing.",
	}, s.dictate)

	mcp.AddTool(server, &mcp.Tool{
		Name: "setup",
		Description: "Install what dictation needs: a prebuilt whisper.cpp engine and a multilingual " +
			"model into Prime's data dir, plus a microphone recorder when none exists. " +
			"Safe to run repeatedly; pieces that already work are left alone.",
	}, s.setup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "status",
		Description: "Report recorders, Whisper engines, the resident server state, and whether dictate is ready to use.",
	}, s.status)

	mcp.AddTool(server, &mcp.Tool{
		Name: "warm",
		Description: "Start the resident Whisper server and load the model so the next dictate answers " +
			"in seconds. No-op when the server is already up.",
	}, s.warm)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stop_server",
		Description: "Stop the resident Whisper server and return its memory. The next dictate or warm starts it again.",
	}, s.stopServer)

	return server
}

func newService() (*service, error) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	store, err := config.Init(cwd, "", false)
	if err != nil {
		return nil, fmt.Errorf("cannot read Prime configuration: %w", err)
	}
	var options *config.VoiceOptions
	if cfg := store.Config(); cfg != nil && cfg.Options != nil {
		options = cfg.Options.Voice
	}
	settings := voiceengine.SettingsFrom(options, store.Resolver())
	return &service{
		settings: settings,
		detector: voiceengine.NewDetector(settings),
	}, nil
}

type dictateInput struct {
	MaxSeconds int    `json:"max_seconds,omitempty" jsonschema:"hard cap for the recording in seconds (default 30, max 300)"`
	SilenceMs  int    `json:"silence_ms,omitempty" jsonschema:"milliseconds of quiet that end the recording (default 700)"`
	Language   string `json:"language,omitempty" jsonschema:"ISO 639-1 code like ru or en, overriding the configured language"`
}

func (s *service) dictate(ctx context.Context, _ *mcp.CallToolRequest, in dictateInput) (*mcp.CallToolResult, any, error) {
	if !s.settings.Enabled {
		return errorResult("voice input is disabled: run `prime config` or set option voice on"), nil, nil
	}

	maxSeconds := in.MaxSeconds
	if maxSeconds <= 0 {
		maxSeconds = dictateDefaultMaxSeconds
	}
	options := voiceengine.DictateOptions{
		MaxDuration: time.Duration(min(maxSeconds, 300)) * time.Second,
	}
	if in.SilenceMs > 0 {
		options.TrailingSilence = time.Duration(in.SilenceMs) * time.Millisecond
	}

	detector := s.detectorFor(in.Language)
	dictation, err := detector.Dictate(ctx, options)
	switch {
	case errors.Is(err, voiceengine.ErrNoRecorder), errors.Is(err, voiceengine.ErrNoTranscriber):
		return errorResult(err.Error() + " — call voice_setup first"), nil, nil
	case errors.Is(err, voiceengine.ErrNoSpeech):
		return errorResult("nothing was heard in time; tell the user to speak right after you call dictate, or raise max_seconds"), nil, nil
	case err != nil:
		return errorResult("dictation failed: " + err.Error()), nil, nil
	}

	return textResult(fmt.Sprintf("%s\n\n[recorded %.1fs via %s/%s]",
		dictation.Text, dictation.Duration.Seconds(), dictation.Recorder, dictation.Engine)), nil, nil
}

// detectorFor reuses the shared cached detector unless the call pins a
// different language, which the pipeline bakes into its transcriber.
func (s *service) detectorFor(language string) *voiceengine.Detector {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if normalized == "" || normalized == s.settings.Language {
		return s.detector
	}
	settings := s.settings
	settings.Language = normalized
	return voiceengine.NewDetector(settings)
}

type setupInput struct {
	Model string `json:"model,omitempty" jsonschema:"model to download: large-v3-turbo-q5_0 (default), tiny, base, small, medium, or a ggml file path to keep using"`
}

func (s *service) setup(ctx context.Context, _ *mcp.CallToolRequest, in setupInput) (*mcp.CallToolResult, any, error) {
	plan, err := s.detector.SetupPlan(ctx, voiceengine.SetupOptions{Model: in.Model})
	if err != nil {
		return errorResult("cannot plan the voice setup: " + err.Error()), nil, nil
	}

	var progress []string
	appendLog := func(line string) { progress = append(progress, strings.TrimSpace(line)) }

	if plan.Empty() {
		s.detector.Invalidate()
		go s.warmNow()
		return textResult(strings.Join(append([]string{"Voice input is already set up:"}, plan.Steps()...), "\n")), nil, nil
	}

	if err := plan.Apply(ctx, appendLog); err != nil {
		report := strings.Join(progress, "\n")
		return errorResult("voice setup stopped: " + err.Error() + "\n" + report), nil, nil
	}

	s.detector.Invalidate()
	go s.warmNow()
	lines := append([]string{"Voice setup finished:"}, progress...)
	lines = append(lines, "Warm the model with voice_warm before the first dictate.")
	return textResult(strings.Join(lines, "\n")), nil, nil
}

func (s *service) status(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	settings := s.detector.Settings()
	recorders, transcribers := s.detector.Backends(ctx)
	server := voiceengine.ServerStatus(ctx)

	var lines []string
	lines = append(lines, "Installed under: "+s.detector.Layout().Root)
	lines = append(lines, "Language: "+orLabel(settings.Language, "auto-detect"))
	lines = append(lines, "Model: "+orLabel(settings.Model, "default"))
	lines = append(lines, "Recorders: "+namesOr(recorders, "none found - run voice_setup"))
	lines = append(lines, "Whisper engines: "+namesOr(transcribers, "none found - run voice_setup"))

	if server.Listening {
		idle := ""
		if server.HasState {
			idle = fmt.Sprintf(", idle for %s", server.IdleFor.Round(time.Second))
		}
		lines = append(lines, fmt.Sprintf("Resident server: listening on %d%s", server.Port, idle))
	} else if server.HasState {
		lines = append(lines, "Resident server: recorded but not answering (pid "+fmt.Sprint(server.State.PID)+")")
	} else {
		lines = append(lines, "Resident server: not running (voice_warm starts it)")
	}

	if len(recorders) > 0 && len(transcribers) > 0 {
		lines = append(lines, "Ready to dictate: yes")
	} else {
		lines = append(lines, "Ready to dictate: no - call voice_setup")
	}
	if !settings.Enabled {
		lines = append(lines, "Voice input is disabled by options.voice.enabled")
	}
	return textResult(strings.Join(lines, "\n")), nil, nil
}

func (s *service) warm(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	s.warmNow()
	server := voiceengine.ServerStatus(ctx)
	if !server.Listening {
		return errorResult("the resident Whisper server is not answering yet; see the log at " + server.LogFile), nil, nil
	}
	model := "unknown"
	if server.HasState {
		model = server.State.Model
	}
	return textResult("Whisper server is warm and ready: " + model), nil, nil
}

func (s *service) stopServer(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	wasLive, err := voiceengine.StopServer(ctx)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	if !wasLive {
		return textResult("no resident Whisper server was running"), nil, nil
	}
	return textResult("the resident Whisper server was stopped"), nil, nil
}

// warmNow starts the resident server so the model is loaded and ready. Failed
// warm-ups stay quiet: dictate warms the same server through its own
// transcriber and reports real errors then.
func (s *service) warmNow() {
	s.mu.Lock()
	if s.warming {
		s.mu.Unlock()
		return
	}
	s.warming = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.warming = false
		s.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := voiceengine.WarmServer(ctx, s.detector.Settings()); err != nil {
		slog.Debug("voice MCP warm-up did not finish", "error", err)
	}
}

func namesOr[T interface{ Name() string }](items []T, empty string) string {
	if len(items) == 0 {
		return empty
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.Name())
	}
	return strings.Join(lines, ", ")
}

func orLabel(value, empty string) string {
	if strings.TrimSpace(value) == "" {
		return empty
	}
	return value
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
