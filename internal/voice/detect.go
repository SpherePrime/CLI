package voice

import (
	"context"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// pythonProbeTimeout bounds the "can this interpreter import sounddevice"
// check, which spawns a process and is therefore the slowest part of detection.
const pythonProbeTimeout = 10 * time.Second

// prober answers "is this tool available" questions. It exists as a seam so
// the detection order can be tested on a machine that has none of the tools,
// and so probes that spawn processes run once per session.
type prober struct {
	lookPath          func(name string) (string, bool)
	pythonSounddevice func(ctx context.Context) (string, bool)
	reachable         func(ctx context.Context, url string) bool
}

// newProber builds the production prober. The Python check is cached because
// it costs a process spawn and would otherwise run on every detection pass.
func newProber() prober {
	look := func(name string) (string, bool) {
		path, err := exec.LookPath(name)
		if err != nil || path == "" {
			return "", false
		}
		return path, true
	}
	return prober{
		lookPath:          look,
		pythonSounddevice: cachedPythonSounddeviceProbe(look),
		reachable:         urlReachable,
	}
}

// cachedPythonSounddeviceProbe returns a probe that runs at most once and
// keeps the interpreter that can capture audio.
func cachedPythonSounddeviceProbe(look func(string) (string, bool)) func(context.Context) (string, bool) {
	var (
		once sync.Once
		path string
	)
	return func(ctx context.Context) (string, bool) {
		once.Do(func() { path = findPythonWithSounddevice(ctx, look) })
		return path, path != ""
	}
}

// pythonCandidates are the interpreters tried in order. Windows ships a
// "python3" shim that opens the Store rather than running code, so the plain
// launcher names come first.
var pythonCandidates = []string{"python", "py", "python3"}

// findPythonWithSounddevice returns the first interpreter whose sounddevice
// module imports cleanly, which is what microphone capture needs.
func findPythonWithSounddevice(ctx context.Context, look func(string) (string, bool)) string {
	ctx, cancel := context.WithTimeout(ctx, pythonProbeTimeout)
	defer cancel()
	for _, candidate := range pythonCandidates {
		interpreter, ok := look(candidate)
		if !ok {
			continue
		}
		probe := exec.CommandContext(ctx, interpreter, "-c", "import numpy, sounddevice")
		if err := probe.Run(); err == nil {
			return interpreter
		}
	}
	return ""
}

// localWhisperServer is where a whisper.cpp server usually waits, which lets
// voice input work with nothing installed beyond the server itself.
const localWhisperServer = "http://127.0.0.1:8000/health"

// urlReachable reports whether an HTTP endpoint answers at all. Any response
// counts, including a 404 from a server that has no health route.
func urlReachable(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return true
}

// Detector resolves the capture and transcription backends once and keeps the
// result. Probing spawns processes and touches the network, which is far too
// slow to repeat on every keypress.
type Detector struct {
	settings Settings
	prober   prober

	mu       sync.Mutex
	plan     *Plan
	planErr  error
	resolved bool
}

// NewDetector returns a detector for the given settings.
func NewDetector(settings Settings) *Detector {
	return &Detector{settings: settings, prober: newProber()}
}

// newDetectorWithProber lets tests drive detection without a machine that has
// every tool installed.
func newDetectorWithProber(settings Settings, p prober) *Detector {
	return &Detector{settings: settings, prober: p}
}

// Settings returns the settings the detector was built with.
func (d *Detector) Settings() Settings {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settings
}

// Invalidate drops the cached plan so the next use re-reads the machine, for
// example after the user installs ffmpeg mid-session.
func (d *Detector) Invalidate() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.plan = nil
	d.planErr = nil
	d.resolved = false
}

// Plan returns the resolved voice pipeline, detecting it on first use.
func (d *Detector) Plan(ctx context.Context) (*Plan, error) {
	d.mu.Lock()
	if d.resolved {
		plan, err := d.plan, d.planErr
		d.mu.Unlock()
		return plan, err
	}
	d.mu.Unlock()

	plan, err := detectPlan(ctx, d.Settings(), d.prober)

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.resolved {
		d.plan, d.planErr, d.resolved = plan, err, true
	}
	return plan, err
}

// Backends lists what this machine offers, for diagnostics.
func (d *Detector) Backends(ctx context.Context) (recorders []Recorder, transcribers []Transcriber) {
	settings := d.Settings()
	return newRecorders(ctx, settings, d.prober), newTranscribers(ctx, settings, d.prober)
}

// detectPlan picks the first usable recorder and transcriber.
func detectPlan(ctx context.Context, settings Settings, p prober) (*Plan, error) {
	recorders := newRecorders(ctx, settings, p)
	if len(recorders) == 0 {
		return nil, ErrNoRecorder
	}
	transcribers := newTranscribers(ctx, settings, p)
	if len(transcribers) == 0 {
		return nil, ErrNoTranscriber
	}
	return &Plan{Recorder: recorders[0], Transcriber: transcribers[0]}, nil
}
