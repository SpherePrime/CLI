package voice

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// stopHow describes how a recorder likes to be told that capture is over.
// Ending politely matters most for tools that write a file themselves, since
// they have to patch the audio header on the way out.
type stopHow int

const (
	// stopKill terminates the process at once, which is enough for recorders
	// streaming raw PCM.
	stopKill stopHow = iota
	// stopStdinQ writes "q" to stdin, the way ffmpeg quits.
	stopStdinQ
	// stopInterrupt sends SIGINT, the way command line recorders end.
	stopInterrupt
)

// pcmOutputArgs are the ffmpeg flags that produce the format Whisper wants.
var pcmOutputArgs = []string{"-ac", "1", "-ar", "16000", "-f", "s16le", "pipe:1"}

// newRecorders returns the capture backends usable on this machine, best
// match first.
func newRecorders(ctx context.Context, settings Settings, p prober) []Recorder {
	if settings.RecordCommand != "" {
		return []Recorder{processRecorder{
			name:     "record-command",
			fileMode: usesOutputPlaceholder(settings.RecordCommand),
			stop:     stopStdinQ,
			build:    shellArgs(settings.RecordCommand),
		}}
	}

	var recorders []Recorder
	// The Python helper needs nothing but "pip install sounddevice", so on
	// Windows it comes before ffmpeg, where capture needs an enumerated
	// device name.
	if runtime.GOOS == "windows" {
		if py, ok := p.pythonSounddevice(ctx); ok {
			recorders = append(recorders, sounddeviceRecorder(py))
		}
	}
	if ffmpeg, ok := p.lookPath("ffmpeg"); ok {
		recorders = append(recorders, ffmpegRecorder(ffmpeg, p))
	}
	if sox, ok := p.lookPath("rec"); ok {
		recorders = append(recorders, soxRecorder(sox))
	}
	switch runtime.GOOS {
	case "linux":
		if pw, ok := p.lookPath("pw-record"); ok {
			recorders = append(recorders, pipewireRecorder(pw))
		}
		if alsa, ok := p.lookPath("arecord"); ok {
			recorders = append(recorders, alsaRecorder(alsa))
		}
	}
	if runtime.GOOS != "windows" {
		if py, ok := p.pythonSounddevice(ctx); ok {
			recorders = append(recorders, sounddeviceRecorder(py))
		}
	}
	return recorders
}

// ffmpegRecorder streams 16 kHz mono PCM from the platform input device.
func ffmpegRecorder(path string, p prober) Recorder {
	return processRecorder{
		name: "ffmpeg",
		stop: stopStdinQ,
		build: func(ctx context.Context, _ string) ([]string, error) {
			input, err := ffmpegInput(ctx, path, p)
			if err != nil {
				return nil, err
			}
			args := []string{path, "-hide_banner", "-loglevel", "error", "-nostats"}
			args = append(args, input...)
			return append(args, pcmOutputArgs...), nil
		},
	}
}

// ffmpegInput picks the device flags for the operating system. Windows has no
// way to name the default dshow device, so the first enumerated microphone is
// used instead.
func ffmpegInput(ctx context.Context, path string, p prober) ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		device, err := firstDShowAudioDevice(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNoRecorder, err)
		}
		return []string{"-f", "dshow", "-i", "audio=" + device}, nil
	case "darwin":
		return []string{"-f", "avfoundation", "-i", ":default"}, nil
	default:
		return []string{"-f", "alsa", "-i", "default"}, nil
	}
}

// soxRecorder uses sox's "rec" front end, which reads the default input.
func soxRecorder(path string) Recorder {
	return processRecorder{
		name:  "sox",
		stop:  stopInterrupt,
		build: staticBuilder(path, "-q", "-b", "16", "-c", "1", "-r", "16000", "-t", "raw", "-"),
	}
}

// pipewireRecorder reads the PipeWire default source into a WAV file.
func pipewireRecorder(path string) Recorder {
	return processRecorder{
		name:     "pw-record",
		fileMode: true,
		stop:     stopInterrupt,
		build:    staticBuilder(path, "--rate=16000", "--channels=1", "--format=s16le"),
	}
}

// alsaRecorder reads the ALSA default device straight to stdout.
func alsaRecorder(path string) Recorder {
	return processRecorder{
		name:  "arecord",
		stop:  stopInterrupt,
		build: staticBuilder(path, "-q", "-t", "raw", "-f", "S16_LE", "-r", "16000", "-c", "1"),
	}
}

// sounddeviceRecorder captures with Python's sounddevice module.
func sounddeviceRecorder(interpreter string) Recorder {
	return processRecorder{
		name: "python-sounddevice",
		stop: stopInterrupt,
		build: func(_ context.Context, _ string) ([]string, error) {
			return []string{interpreter, "-u", "-c", sounddeviceScript}, nil
		},
	}
}

// staticBuilder builds an argv from a fixed binary and flags, appending the
// output path for backends that write a file.
func staticBuilder(path string, flags ...string) captureBuilder {
	return func(_ context.Context, outPath string) ([]string, error) {
		argv := append([]string{path}, flags...)
		if outPath != "" {
			argv = append(argv, outPath)
		}
		return argv, nil
	}
}

// sounddeviceScript captures the default input device and writes 16 kHz mono
// PCM to stdout. The device is opened at its own rate and channel count
// because many microphones refuse 16 kHz outright, so the script mixes down to
// mono and resamples on its own.
const sounddeviceScript = `
import sys, time
try:
    import numpy as np
    import sounddevice as sd
except Exception as exc:
    sys.stderr.write("sounddevice is required: pip install sounddevice (%s)\n" % exc)
    sys.exit(3)

TARGET = 16000
out = sys.stdout.buffer
device = sd.query_devices(kind="input")
rate = int(device["default_samplerate"] or TARGET)
channels = int(device.get("max_input_channels") or 1)
ratio = rate / float(TARGET)

def callback(indata, frames, time_info, status):
    if status:
        sys.stderr.write("microphone: %s\n" % status)
    data = np.asarray(indata)
    mono = data.mean(axis=-1) if data.ndim > 1 else data
    mono = mono.astype("float32")
    if ratio != 1.0:
        count = int(mono.shape[0] / ratio)
        if count <= 0:
            return
        pos = np.linspace(0, mono.shape[0] - 1, count)
        low = np.floor(pos).astype(np.int64)
        high = np.minimum(low + 1, mono.shape[0] - 1)
        frac = (pos - low).astype("float32")
        mono = mono[low] * (1 - frac) + mono[high] * frac
    out.write(np.clip(mono, -32768.0, 32767.0).astype("<i2").tobytes())
    out.flush()

try:
    with sd.InputStream(callback=callback, dtype="int16", channels=channels, samplerate=rate):
        while True:
            time.sleep(0.05)
except KeyboardInterrupt:
    pass
except Exception as exc:
    sys.stderr.write("microphone capture failed: %s\n" % exc)
    sys.exit(4)`

var (
	dshowAudioSection = regexp.MustCompile(`Direct audio devices`)
	dshowQuotedName   = regexp.MustCompile(`^\s*"(.+)"\s+\(audio\)\s*$`)
	dshowNumberedName = regexp.MustCompile(`^\s*\d+\.\s+(.+?)\s*$`)
	dshowStreamPrefix = regexp.MustCompile(`^\s*\[dshow[^\]]*\]\s*(.*)$`)
)

// firstDShowAudioDevice asks ffmpeg for its device list, which is the only way
// to name the default microphone on Windows.
func firstDShowAudioDevice(ctx context.Context, ffmpegPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-loglevel", "info", "-list_devices", "true", "-f", "dshow", "-i", "dummy").CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("cannot list audio devices: %w", err)
	}
	devices := parseDShowAudioDevices(string(out))
	if len(devices) == 0 {
		return "", fmt.Errorf("ffmpeg reports no audio input devices")
	}
	return devices[0], nil
}

// parseDShowAudioDevices pulls audio device names out of ffmpeg's dshow
// listing. ffmpeg has used two shapes over time, a quoted name suffixed with
// "(audio)" and an index in front of the name inside a "Direct audio devices"
// section, so both are accepted.
func parseDShowAudioDevices(out string) []string {
	var devices []string
	inAudioSection := false
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.Contains(line, "Direct video devices"):
			inAudioSection = false
			continue
		case dshowAudioSection.MatchString(line):
			inAudioSection = true
			continue
		}
		body := line
		if match := dshowStreamPrefix.FindStringSubmatch(line); match != nil {
			body = match[1]
		} else if !inAudioSection {
			continue
		}
		if match := dshowQuotedName.FindStringSubmatch(body); match != nil {
			devices = append(devices, match[1])
			continue
		}
		if !inAudioSection {
			continue
		}
		if match := dshowNumberedName.FindStringSubmatch(body); match != nil {
			devices = append(devices, strings.Trim(match[1], `"`))
		}
	}
	return devices
}
