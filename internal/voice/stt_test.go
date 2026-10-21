package voice

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestSttCatalogShape checks the catalog invariants the desktop relies on:
// unique model ids, reachable-looking download URLs, one recommended
// whisper model, and the small vosk models being the recommended ones.
func TestSttCatalogShape(t *testing.T) {
	providers := SttCatalog(t.TempDir())
	require.Len(t, providers, 3)

	byID := map[string]SttProvider{}
	for _, provider := range providers {
		_, duplicate := byID[provider.ID]
		require.False(t, duplicate, "duplicate provider %s", provider.ID)
		byID[provider.ID] = provider
	}

	google := byID[SttGoogle]
	require.True(t, google.Available)
	require.Empty(t, google.Models)

	whisper := byID[SttWhisper]
	require.True(t, whisper.Available, "linux amd64 has a whisper runtime")
	recommended := 0
	seen := map[string]bool{}
	for _, model := range whisper.Models {
		require.False(t, seen[model.ID], "duplicate whisper model %s", model.ID)
		seen[model.ID] = true
		require.True(t, strings.HasPrefix(whisperModelURLPrefix+model.ID+".bin", "https://"))
		require.Greater(t, model.SizeBytes, int64(0))
		if model.Recommended {
			recommended++
		}
	}
	require.Equal(t, 1, recommended, "whisper must recommend exactly one model")

	vosk := byID[SttVosk]
	require.True(t, vosk.Available)
	voskSeen := map[string]bool{}
	for _, model := range vosk.Models {
		require.False(t, voskSeen[model.ID], "duplicate vosk model %s", model.ID)
		voskSeen[model.ID] = true
		require.True(t, strings.HasPrefix(voskModelURLPrefix+model.ID+".zip", "https://"))
		require.NotEmpty(t, model.Language)
		if model.Recommended {
			require.True(t, strings.Contains(model.ID, "-small-"),
				"vosk recommendations must be the small models")
		}
	}
}

// sttTestServer serves the payloads the installer downloads, mapped through
// the URL rewrite table. The archives are tiny stand-ins for the real
// multi-hundred-megabyte payloads.
func sttTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/hf/"):
			// Stand-in GGML model file.
			_, _ = w.Write(bytes.Repeat([]byte("ggml"), 1024))
		case strings.Contains(r.URL.Path, "/whisper-runtime/"):
			// The whisper.cpp Linux archive, whose binary entry is called
			// main here to exercise the fallback detection.
			tarGzWrite(t, w, map[string][]byte{"main": []byte("binary")})
		case strings.Contains(r.URL.Path, "/vosk-runtime/"):
			// Vosk runtime: one entry per platform's library name.
			zipWrite(t, w, map[string][]byte{
				"libvosk.so":    []byte("elf"),
				"vosk.dll":      []byte("mz"),
				"libvosk.dylib": []byte("mach-o"),
			})
		case strings.Contains(r.URL.Path, "/vosk-models/"):
			// Vosk model: the zip ships a top-level model folder.
			name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
			zipWrite(t, w, map[string][]byte{
				name + "/conf/model.conf": []byte("conf"),
				name + "/am/final.mdl":    []byte("model"),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// zipWrite sends a zip archive of files over w.
func zipWrite(t *testing.T, w http.ResponseWriter, files map[string][]byte) {
	t.Helper()
	buf := &bytes.Buffer{}
	writer := zip.NewWriter(buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	w.Header().Set("Content-Type", "application/zip")
	_, _ = w.Write(buf.Bytes())
}

// tarGzWrite sends a gzipped tarball of files over w.
func tarGzWrite(t *testing.T, w http.ResponseWriter, files map[string][]byte) {
	t.Helper()
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	writer := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}))
		_, err := writer.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, gz.Close())
	w.Header().Set("Content-Type", "application/gzip")
	_, _ = w.Write(buf.Bytes())
}

// pointSttDownloadsAt redirects every official download URL this test needs
// at the local server and restores the table afterwards.
func pointSttDownloadsAt(t *testing.T, serverURL string) {
	t.Helper()
	setSttURLRewrites(map[string]string{
		whisperModelURLPrefix: serverURL + "/hf/",
		"https://github.com/ggml-org/whisper.cpp/releases/download/b5454/": serverURL + "/whisper-runtime/",
		voskModelURLPrefix: serverURL + "/vosk-models/",
		"https://github.com/alphacep/vosk-api/releases/download/v0.3.45/": serverURL + "/vosk-runtime/",
		"https://github.com/alphacep/vosk-api/releases/download/v0.3.42/": serverURL + "/vosk-runtime/",
	})
	t.Cleanup(func() { setSttURLRewrites(map[string]string{}) })
}

// TestSttInstallLifecycle walks install and remove for one whisper and one
// vosk model against fake download servers.
func TestSttInstallLifecycle(t *testing.T) {
	server := sttTestServer(t)
	pointSttDownloadsAt(t, server.URL)
	dataDir := t.TempDir()

	var progresses []SetupProgress
	emit := func(progress SetupProgress) { progresses = append(progresses, progress) }

	// Whisper: runtime tar.gz plus the model file.
	require.NoError(t, SttInstall(context.Background(), dataDir, SttWhisper, "ggml-tiny", emit))
	require.True(t, SttIsInstalled(dataDir, SttWhisper, "ggml-tiny"))
	require.FileExists(t, whisperModelPath(dataDir, "ggml-tiny"))
	require.NotEmpty(t, findWhisperBinary(whisperBinDir(dataDir)))
	require.True(t, hasProgress(t, progresses, "download"))

	// Idempotent reinstall.
	before := len(progresses)
	require.NoError(t, SttInstall(context.Background(), dataDir, SttWhisper, "ggml-tiny", emit))
	require.True(t, hasProgress(t, progresses[before:], "ready"))

	// Vosk: runtime zip plus the model zip.
	require.NoError(t, SttInstall(context.Background(), dataDir, SttVosk, "vosk-model-small-en-us-0.15", emit))
	require.True(t, SttIsInstalled(dataDir, SttVosk, "vosk-model-small-en-us-0.15"))
	require.FileExists(t, voskLibPath(dataDir))
	require.FileExists(t, filepath.Join(voskModelDir(dataDir, "vosk-model-small-en-us-0.15"), "am", "final.mdl"))

	// Remove leaves the runtimes but clears the models.
	require.NoError(t, SttRemove(dataDir, SttWhisper, "ggml-tiny"))
	require.NoError(t, SttRemove(dataDir, SttVosk, "vosk-model-small-en-us-0.15"))
	require.False(t, SttIsInstalled(dataDir, SttWhisper, "ggml-tiny"))
	require.False(t, SttIsInstalled(dataDir, SttVosk, "vosk-model-small-en-us-0.15"))
	require.NotEmpty(t, findWhisperBinary(whisperBinDir(dataDir)))
	require.FileExists(t, voskLibPath(dataDir))
}

// hasProgress reports whether any update used the phase.
func hasProgress(t *testing.T, progresses []SetupProgress, phase string) bool {
	t.Helper()
	for _, progress := range progresses {
		if progress.Phase == phase {
			return true
		}
	}
	return false
}

// TestSttDownloadProgress reports monotonic byte counts, which the
// progress bar depends on.
func TestSttDownloadProgress(t *testing.T) {
	server := sttTestServer(t)
	pointSttDownloadsAt(t, server.URL)
	dest := filepath.Join(t.TempDir(), "model.bin")

	var lastDone int64
	err := downloadToFile(context.Background(), sttDownloadURL(whisperModelURLPrefix+"ggml-tiny.bin"),
		dest, "Downloading ggml-tiny", func(progress SetupProgress) {
			require.GreaterOrEqual(t, progress.Done, lastDone)
			lastDone = progress.Done
		})
	require.NoError(t, err)
	require.Greater(t, lastDone, int64(0))
	require.FileExists(t, dest)
}

// TestSafeExtractTargetRejectsTraversal pins the malicious-archive guard.
func TestSafeExtractTargetRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	for _, name := range []string{
		"../escape.txt",
		"a/../../escape.txt",
		"/etc/passwd",
		"../../etc/passwd",
		"sub/../../../escape",
	} {
		target, err := safeExtractTarget(dest, name)
		require.Error(t, err, name)
		require.Empty(t, target, name)
	}
	// A well-formed relative entry resolves inside dest.
	target, err := safeExtractTarget(dest, "vosk-model-x/am/final.mdl")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(target, dest+string(os.PathSeparator)))
}

// TestDecodeVoskResult covers the result JSON the FFI hands back.
func TestDecodeVoskResult(t *testing.T) {
	require.Equal(t, "hello world", decodeVoskResult([]byte(`{"text":"hello world"}`)))
	require.Equal(t, "", decodeVoskResult([]byte(`{"text":""}`)))
	require.Equal(t, "", decodeVoskResult([]byte(`{"partial":"..."}`)))
	require.Equal(t, "", decodeVoskResult([]byte("not json")))
}

// TestWhisperArgsAndLanguage pins the command construction.
func TestWhisperArgsAndLanguage(t *testing.T) {
	args := whisperArgs("/bin/whisper-cli", "/models/m.bin", "ru", "/tmp/a.wav")
	require.Equal(t, []string{"/bin/whisper-cli", "-m", "/models/m.bin", "-l", "ru", "-nt", "-f", "/tmp/a.wav"}, args)
	require.Equal(t, "auto", whisperLanguage(""))
	require.Equal(t, "ru-ru", whisperLanguage("ru-RU,en-US"))
}

// TestWhisperTranscribeMissingModel checks the clear setup error instead of
// an auto-download: installation never happens inside Transcribe.
func TestWhisperTranscribeMissingModel(t *testing.T) {
	transcriber := newWhisperTranscriber(Settings{Engine: EngineWhisper, Model: "ggml-tiny", DataDir: t.TempDir()})
	_, err := transcriber.Transcribe(context.Background(), &Audio{WAV: encodeWAV(make([]byte, 4096)), Duration: time.Second})
	require.ErrorContains(t, err, "whisper model not installed")
}

// TestVoskTranscribeMissingModel checks the sibling error for vosk.
func TestVoskTranscribeMissingModel(t *testing.T) {
	transcriber := newVoskTranscriber(Settings{Engine: EngineVosk, Model: "vosk-model-small-ru-0.22", DataDir: t.TempDir()})
	_, err := transcriber.Transcribe(context.Background(), &Audio{WAV: encodeWAV(make([]byte, 4096)), Duration: time.Second})
	require.ErrorContains(t, err, "vosk model not installed")
}
