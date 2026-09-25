package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runCLIResult holds the outcome of one command line Whisper run.
type runCLIResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// cliTranscriber is the shared shape of the command line Whisper engines: run
// a tool over a temporary WAV file and read the text it produced.
type cliTranscriber struct {
	label    string
	bin      string
	settings Settings
	// argv builds the tool's arguments for one recording.
	argv func(t cliTranscriber, wavPath, dir string) []string
	// result names the text file the engine writes, relative to dir.
	result func(wavPath, dir string) string
	// env carries extra environment entries, needed when the engine was
	// installed by Prime and must find the libraries beside it.
	env []string
}

func (t cliTranscriber) Name() string { return t.label }

func (t cliTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	dir, err := os.MkdirTemp("", "prime-voice-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	wavPath := filepath.Join(dir, "dictation.wav")
	if err := os.WriteFile(wavPath, audio.WAV, 0o600); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}

	args := t.argv(t, wavPath, dir)
	stdout, stderr, err := runCommandWithEnv(ctx, t.bin, args, mergeEnv(t.env))
	if err != nil {
		return "", engineError(err, stderr)
	}

	if result := t.result(wavPath, dir); result != "" {
		if data, readErr := os.ReadFile(result); readErr == nil {
			return tidyTranscript(string(data)), nil
		}
	}
	if text := tidyTranscript(stdout); text != "" {
		return text, nil
	}
	return "", fmt.Errorf("%w: %s wrote no transcript", ErrTranscribeFailed, t.label)
}

// engineError explains a failed engine run, keeping the tool's own complaint
// because that is usually the only hint about a missing model or a CUDA driver.
func engineError(err error, stderr string) error {
	if stderr != "" {
		return fmt.Errorf("%w: %v: %s", ErrTranscribeFailed, err, stderr)
	}
	return fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
}

// dictationStem is the name engines derive their output files from.
const dictationStem = "dictation"

// whisperCPPTranscriber runs whisper.cpp, which needs a model file rather than
// a model name.
func whisperCPPTranscriber(bin, model string, settings Settings, env []string) Transcriber {
	return cliTranscriber{
		label:    "whisper.cpp",
		bin:      bin,
		settings: settings,
		env:      env,
		argv: func(t cliTranscriber, wavPath, dir string) []string {
			// whisper.cpp defaults to English, so detection is asked for
			// explicitly whenever the user has not pinned a language.
			language := settings.Language
			if language == "" {
				language = "auto"
			}
			return []string{
				"-f", wavPath,
				"-m", model,
				"-l", language,
				"-nt", "-np",
				"-ot", "-of", filepath.Join(dir, dictationStem),
			}
		},
		result: func(_, dir string) string {
			return filepath.Join(dir, dictationStem+".txt")
		},
	}
}

// pythonWhisperTranscriber runs the openai-whisper command line tool.
func pythonWhisperTranscriber(bin string, settings Settings) Transcriber {
	return cliTranscriber{
		label:    "openai-whisper",
		bin:      bin,
		settings: settings,
		argv: func(t cliTranscriber, wavPath, dir string) []string {
			args := []string{
				wavPath,
				"--model", t.settings.modelOrDefault(),
				"--output_format", "txt",
				"--output_dir", dir,
				"--fp16", "False",
			}
			if t.settings.Language != "" {
				args = append(args, "--language", t.settings.Language)
			}
			return args
		},
		result: func(_, dir string) string {
			return filepath.Join(dir, dictationStem+".txt")
		},
	}
}

// ctranslate2Transcriber runs the faster-whisper based CTranslate2 CLI.
func ctranslate2Transcriber(bin string, settings Settings) Transcriber {
	return cliTranscriber{
		label:    "whisper-ctranslate2",
		bin:      bin,
		settings: settings,
		argv: func(t cliTranscriber, wavPath, dir string) []string {
			args := []string{
				wavPath,
				"--model", t.settings.modelOrDefault(),
				"--output_format", "txt",
				"--output_dir", dir,
			}
			if t.settings.Language != "" {
				args = append(args, "--language", t.settings.Language)
			}
			return args
		},
		result: func(_, dir string) string {
			return filepath.Join(dir, dictationStem+".txt")
		},
	}
}

// commandTranscriber runs options.voice.transcribe-command, whose stdout is
// taken as the transcript.
type commandTranscriber struct {
	command string
	lang    string
}

func (t commandTranscriber) Name() string { return "transcribe-command" }

func (t commandTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	file, err := os.CreateTemp("", "prime-voice-dictation-*.wav")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	if _, err := file.Write(audio.WAV); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}

	args, err := commandTranscribeArgs(t.command, path)
	if err != nil {
		return "", err
	}

	stdout, stderr, err := runCommandWithEnv(ctx, args[0], args[1:], commandTranscribeEnv(t.lang))
	if err != nil {
		return "", engineError(err, stderr)
	}
	return tidyTranscript(stdout), nil
}

// commandTranscribeArgs turns options.voice.transcribe-command into argv. The
// audio file replaces %s, and is appended when the command does not name a
// placeholder.
func commandTranscribeArgs(command string, wavPath string) ([]string, error) {
	fields, err := shellFields(command)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: empty transcription command", ErrTranscribeFailed)
	}
	args := make([]string, 0, len(fields))
	for _, field := range fields {
		args = append(args, strings.ReplaceAll(field, "%s", wavPath))
	}
	if !strings.Contains(command, "%s") {
		args = append(args, wavPath)
	}
	return args, nil
}

// commandTranscribeEnv hands a custom engine the recording format, so a script
// can react to the language without parsing Prime's arguments.
func commandTranscribeEnv(language string) []string {
	env := os.Environ()
	if language != "" {
		env = append(env, "PRIME_VOICE_LANGUAGE="+language)
	}
	return append(env, "PRIME_VOICE_SAMPLE_RATE=16000")
}

// ggmlModelFile resolves a whisper.cpp model, which is addressed by file
// rather than by name. When no name is configured, any model that exists is
// used, so a model installed by "prime voice setup" works without settings.
func ggmlModelFile(configured string, searchDirs []string) (string, bool) {
	if configured != "" && filepath.IsAbs(configured) {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, true
		}
		return "", false
	}
	for _, dir := range searchDirs {
		for _, name := range modelCandidateNames(configured) {
			for _, candidate := range modelFileCandidates(name) {
				path := filepath.Join(dir, candidate)
				if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
					return path, true
				}
			}
		}
	}
	return "", false
}

// modelCandidateNames is the order model names are tried in: the configured one
// first, then the model setup downloads by default, then Prime's fallback and
// every other downloadable model.
func modelCandidateNames(configured string) []string {
	names := make([]string, 0, 3+len(SetupModels))
	seen := make(map[string]bool, 3+len(SetupModels))
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	add(configured)
	add(DefaultSetupModel)
	add(defaultModel)
	for _, model := range SetupModels {
		add(model.Name)
	}
	return names
}

// modelFileCandidates are the file names one model can appear under.
func modelFileCandidates(name string) []string {
	candidates := []string{"ggml-" + name + ".bin", name + ".bin"}
	if !strings.HasSuffix(name, ".bin") {
		candidates = append(candidates, name)
	}
	return candidates
}

// modelSearchDirs is where a ggml model file is looked for.
func modelSearchDirs() []string {
	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd, filepath.Join(cwd, "models"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".cache", "whisper.cpp"),
			filepath.Join(home, "whisper.cpp", "models"),
			filepath.Join(home, "Downloads"),
		)
	}
	if cached := os.Getenv("XDG_CACHE_HOME"); cached != "" {
		dirs = append(dirs, filepath.Join(cached, "whisper.cpp"))
	}
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "Library", "Caches", "whisper.cpp"))
		}
	}
	return dirs
}
