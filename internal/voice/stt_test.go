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
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestSttCatalogShape checks the catalog invariants the desktop relies on:
// unique model ids, reachable-looking download URLs, exactly one recommended
// whisper model, the small vosk models being the recommended ones, and the
// whisper ".en" variants declaring their language.
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
	require.True(t, slices.ContainsFunc(whisper.Models, func(model SttModelInfo) bool {
		return model.ID == "ggml-large-v3-turbo-q5_0" && model.Recommended
	}), "the recommended whisper model must be ggml-large-v3-turbo-q5_0")
	require.Len(t, whisper.Models, 15)
	seen := map[string]bool{}
	recommended := 0
	for _, model := range whisper.Models {
		require.False(t, seen[model.ID], "duplicate whisper model %s", model.ID)
		seen[model.ID] = true
		require.True(t, strings.HasPrefix(whisperModelURLPrefix+model.ID+".bin", "https://"))
		require.Greater(t, model.SizeBytes, int64(0))
		require.NotEmpty(t, model.Label, model.ID)
		// English-only variants are labeled with ".en" ids and declare the
		// language so the desktop can filter them; the rest stay multilingual.
		if strings.HasSuffix(model.ID, ".en") {
			require.Equal(t, "en", model.Language, model.ID)
		} else {
			require.Empty(t, model.Language, model.ID)
		}
		if model.Recommended {
			recommended++
		}
	}
	require.Equal(t, 1, recommended, "whisper must recommend exactly one model")

	vosk := byID[SttVosk]
	require.True(t, vosk.Available)
	require.Len(t, vosk.Models, 9)
	voskSeen := map[string]bool{}
	for _, model := range vosk.Models {
		require.False(t, voskSeen[model.ID], "duplicate vosk model %s", model.ID)
		voskSeen[model.ID] = true
		require.True(t, strings.HasPrefix(voskModelURLPrefix+model.ID+".zip", "https://"))
		require.NotEmpty(t, model.Language)
		require.NotEmpty(t, model.Label, model.ID)
		// The small models are the recommendations, one per language.
		require.Equal(t, strings.Contains(model.ID, "-small-"), model.Recommended,
			"vosk recommendations must be exactly the small models: %s", model.ID)
	}
	require.Equal(t, 9, countIf(vosk.Models, func(model SttModelInfo) bool { return model.Recommended }))
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
			if strings.HasSuffix(r.URL.Path, ".zip") {
				zipWrite(t, w, map[string][]byte{"whisper-cli.exe": []byte("binary")})
				return
			}
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

	// Whisper: runtime tar.gz plus the model file. The empty accelerator
	// is the unconfigured default, which means the CPU build.
	require.NoError(t, SttInstall(context.Background(), dataDir, SttWhisper, "ggml-tiny", "", emit))
	require.True(t, SttIsInstalled(dataDir, SttWhisper, "ggml-tiny"))
	require.FileExists(t, whisperModelPath(dataDir, "ggml-tiny"))
	require.NotEmpty(t, findWhisperBinary(whisperBinDir(dataDir)))
	require.True(t, hasProgress(t, progresses, "download"))

	// Idempotent reinstall.
	before := len(progresses)
	require.NoError(t, SttInstall(context.Background(), dataDir, SttWhisper, "ggml-tiny", "", emit))
	require.True(t, hasProgress(t, progresses[before:], "ready"))

	// Vosk: runtime zip plus the model zip.
	require.NoError(t, SttInstall(context.Background(), dataDir, SttVosk, "vosk-model-small-en-us-0.15", "", emit))
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

// countIf counts the entries a predicate accepts.
func countIf(models []SttModelInfo, match func(SttModelInfo) bool) int {
	count := 0
	for _, model := range models {
		if match(model) {
			count++
		}
	}
	return count
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
	require.Equal(t, []string{"/bin/whisper-cli", "-m", "/models/m.bin", "-l", "ru", "-t", strconv.Itoa(whisperThreads), "-nt", "-f", "/tmp/a.wav"}, args)
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

// TestParseNvidiaSmiOutput pins the row parser: it keeps well-formed rows,
// skips broken ones, and reads the CSV the tool actually prints.
func TestParseNvidiaSmiOutput(t *testing.T) {
	require.Equal(t, []GpuInfo{
		{Index: 0, Name: "NVIDIA GeForce RTX 3070"},
		{Index: 1, Name: "NVIDIA GeForce RTX 5090"},
	}, parseNvidiaSmiOutput([]byte("0, NVIDIA GeForce RTX 3070\r\n1, NVIDIA GeForce RTX 5090\r\n")))
	// Broken rows are skipped, not fatal: one bad line must not hide the
	// GPUs the driver did report.
	require.Equal(t, []GpuInfo{{Index: 2, Name: "Tesla T4"}},
		parseNvidiaSmiOutput([]byte("\nnot a csv row\nnope\n2, Tesla T4\n-x, bad index\n")))
	require.Nil(t, parseNvidiaSmiOutput([]byte("")))
}

// nvidiaSmiStub writes a fake nvidia-smi that prints a fixed GPU list and
// returns its path. The flavor depends on the platform, because the file has
// to be executable the way that platform executes things.
func nvidiaSmiStub(t *testing.T, rows ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "nvidia-smi")
	if runtime.GOOS == "windows" {
		path += ".cmd"
	}
	var body string
	if runtime.GOOS == "windows" {
		body = "@echo off\r\n"
		for _, row := range rows {
			body += "echo " + row + "\r\n"
		}
	} else {
		body = "#!/bin/sh\n"
		for _, row := range rows {
			body += "echo " + strconv.Quote(row) + "\n"
		}
	}
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(path, 0o755))
	}
	return path
}

// TestSttNvidiaGpusPointsAtStubbedTool drives the whole enumeration through
// a stubbed command path: found GPUs are parsed, a missing tool reports no
// GPUs.
func TestSttNvidiaGpusPointsAtStubbedTool(t *testing.T) {
	restore := SetNvidiaSmiForTest(nvidiaSmiStub(t, "0, NVIDIA GeForce RTX 3070", "1, NVIDIA GeForce RTX 5090"))
	defer restore()
	require.Equal(t, []GpuInfo{
		{Index: 0, Name: "NVIDIA GeForce RTX 3070"},
		{Index: 1, Name: "NVIDIA GeForce RTX 5090"},
	}, SttNvidiaGpus())

	// A tool that is not on PATH means no GPUs, not an error.
	restore = SetNvidiaSmiForTest("prime-no-such-nvidia-smi")
	defer restore()
	require.Nil(t, SttNvidiaGpus())
}

// TestInstalledWhisperAcceleratorMarker pins the marker semantics: the file
// wins when it names a GPU accelerator, everything else (missing, CPU,
// garbage) is CPU, because every runtime unpacked before the marker existed
// was the CPU build.
func TestInstalledWhisperAcceleratorMarker(t *testing.T) {
	dataDir := t.TempDir()
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir))
	require.NoError(t, writeWhisperAcceleratorMarker(dataDir, AcceleratorCUDA))
	require.Equal(t, AcceleratorCUDA, installedWhisperAccelerator(dataDir))
	require.NoError(t, writeWhisperAcceleratorMarker(dataDir, AcceleratorVulkan))
	require.Equal(t, AcceleratorVulkan, installedWhisperAccelerator(dataDir))
	require.NoError(t, writeWhisperAcceleratorMarker(dataDir, AcceleratorCPU))
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir))
	require.NoError(t, writeWhisperAcceleratorMarker(dataDir, "  cuda  \n"))
	require.Equal(t, AcceleratorCUDA, installedWhisperAccelerator(dataDir))
	require.NoError(t, writeWhisperAcceleratorMarker(dataDir, "rocm"))
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir), "unknown markers are CPU")
}

// whisperAcceleratorServer serves two different whisper runtime archives
// under the official release prefix, so the test can tell which build the
// installer fetched, and counts runtime requests.
func whisperAcceleratorServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/hf/"):
			_, _ = w.Write(bytes.Repeat([]byte("ggml"), 1024))
		case strings.Contains(r.URL.Path, "/runtime/"):
			requests.Add(1)
			// Each variant ships a different binary payload, so a successful
			// re-extraction is provable from disk.
			payload := []byte("cpu-binary")
			if strings.Contains(r.URL.Path, "cuda") {
				payload = []byte("cuda-binary")
			}
			zipWrite(t, w, map[string][]byte{"main": payload})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// TestWhisperAcceleratorSwitchReinstalls walks the accelerator lifecycle:
// the CPU runtime installs once, a matching accelerator is reused without a
// download, and switching to CUDA re-downloads and re-extracts, leaving the
// marker pointing at the build that is actually on disk.
func TestWhisperAcceleratorSwitchReinstalls(t *testing.T) {
	server, requests := whisperAcceleratorServer(t)
	setSttURLRewrites(map[string]string{
		whisperModelURLPrefix: server.URL + "/hf/",
		"https://github.com/ggml-org/whisper.cpp/releases/download/b5454/": server.URL + "/runtime/",
	})
	t.Cleanup(func() { setSttURLRewrites(map[string]string{}) })

	// Both variants exist for this platform, so the switch below behaves the
	// same way everywhere the tests run.
	originalRuntimes := whisperRuntimes
	whisperRuntimes = []struct{ goos, goarch, accelerator, url string }{
		{runtime.GOOS, runtime.GOARCH, AcceleratorCPU, "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-x64.zip"},
		{runtime.GOOS, runtime.GOARCH, AcceleratorCUDA, "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-win-cuda-12.4.0-x64.zip"},
	}
	t.Cleanup(func() { whisperRuntimes = originalRuntimes })

	dataDir := t.TempDir()
	ctx := context.Background()

	// First install fetches the CPU runtime and records it.
	require.NoError(t, SttInstall(ctx, dataDir, SttWhisper, "ggml-tiny", AcceleratorCPU, func(SetupProgress) {}))
	require.Equal(t, int64(1), requests.Load())
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir))
	cpuBinary, err := os.ReadFile(findWhisperBinary(whisperBinDir(dataDir)))
	require.NoError(t, err)
	require.Equal(t, "cpu-binary", string(cpuBinary))

	// The same accelerator again reuses the runtime: only the model moves.
	require.NoError(t, SttInstall(ctx, dataDir, SttWhisper, "ggml-base", AcceleratorCPU, func(SetupProgress) {}))
	require.Equal(t, int64(1), requests.Load())

	// Switching to CUDA must re-download, because the CPU binary cannot use
	// the GPU, and the stale build must not linger next to the new one.
	require.NoError(t, SttInstall(ctx, dataDir, SttWhisper, "ggml-base", AcceleratorCUDA, func(SetupProgress) {}))
	require.Equal(t, int64(2), requests.Load())
	require.Equal(t, AcceleratorCUDA, installedWhisperAccelerator(dataDir))
	cudaBinary, err := os.ReadFile(findWhisperBinary(whisperBinDir(dataDir)))
	require.NoError(t, err)
	require.Equal(t, "cuda-binary", string(cudaBinary))

	// The switched accelerator is now the installed one: no more downloads.
	require.NoError(t, SttInstall(ctx, dataDir, SttWhisper, "ggml-base", AcceleratorCUDA, func(SetupProgress) {}))
	require.Equal(t, int64(2), requests.Load())

	// An unconfigured accelerator means CPU again, and the CPU build comes
	// back to replace the CUDA one.
	require.NoError(t, SttInstall(ctx, dataDir, SttWhisper, "ggml-base", "", func(SetupProgress) {}))
	require.Equal(t, int64(3), requests.Load())
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir))
}

// TestWhisperEnv pins when the transcriber adds device-pinning environment
// and the loader path: the library directory is always handed to the dynamic
// loader on unix, and an explicit GPU accelerator with a chosen device adds
// the variable that backend speaks.
func TestWhisperEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the loader path is Windows-exe-directory semantics")
	}
	env := func(settings Settings) []string {
		return whisperEnv(settings, "/runtime/bin")
	}
	for _, settings := range []Settings{
		{}, {Accelerator: AcceleratorCPU}, {Accelerator: AcceleratorCUDA}, {Accelerator: AcceleratorVulkan},
	} {
		list := env(settings)
		require.NotEmpty(t, list)
		require.Contains(t, list, "LD_LIBRARY_PATH=/runtime/bin")
	}

	list := env(Settings{Accelerator: AcceleratorCUDA, GPUDevice: "1"})
	require.Contains(t, list, "CUDA_VISIBLE_DEVICES=1")
	require.Contains(t, list, "LD_LIBRARY_PATH=/runtime/bin")
	// The rest of the environment survives, because whisper.cpp needs PATH
	// and friends like any other child process.
	require.Greater(t, len(list), 2)

	list = env(Settings{Accelerator: AcceleratorVulkan, GPUDevice: "2"})
	require.Contains(t, list, "GGML_VK_VISIBLE_DEVICES=2")
	require.Contains(t, list, "LD_LIBRARY_PATH=/runtime/bin")
	require.Greater(t, len(list), 2)
}

// TestWhisperAcceleratorFallsBackToCPU pins the runtime resolution: a
// requested accelerator without a published build for this platform falls
// back to the CPU binary instead of failing the install.
func TestWhisperAcceleratorFallsBackToCPU(t *testing.T) {
	url, variant, ok := whisperRuntime(AcceleratorCPU)
	require.True(t, ok)
	require.NotEmpty(t, url)
	require.Equal(t, AcceleratorCPU, variant)

	// A CUDA build is published for windows/amd64 only; everywhere else the
	// request degrades to the CPU variant.
	url, variant, ok = whisperRuntime(AcceleratorCUDA)
	require.True(t, ok, "the CPU fallback always answers")
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		require.Equal(t, AcceleratorCUDA, variant)
		require.Contains(t, url, "cuda")
	} else {
		require.Equal(t, AcceleratorCPU, variant)
	}

	// The catalog offer mirrors the table: CUDA only where its build exists.
	offers := WhisperAccelerators()
	require.Equal(t, []string{AcceleratorCPU}, offers[:1])
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		require.Equal(t, []string{AcceleratorCPU, AcceleratorCUDA}, offers)
	} else {
		require.Equal(t, []string{AcceleratorCPU}, offers)
	}
}

// TestParseVulkanInfoOutput pins the vulkaninfo --summary parser on a
// realistic Devices block: GPUs are read in order, the llvmpipe software
// renderer is vetoed by its CPU device type, and entries without a usable
// name do not hide the devices that did parse.
func TestParseVulkanInfoOutput(t *testing.T) {
	summary := `==========
VULKAN INFO
==========

Vulkan Instance Version: 1.3.280

Devices:
========
	GPU0:
		apiVersion     = 1.3.280
		driverVersion  = 1
		vendorID       = 0x1002
		deviceID       = 0x744c
		deviceType     = DISCRETE_GPU
		deviceName     = AMD Radeon RX 7900 XT
	GPU1:
		apiVersion     = 1.3.280
		driverVersion  = 1
		vendorID       = 0x8086
		deviceID       = 0x0000
		deviceType     = INTEGRATED_GPU
		deviceName     = Intel(R) Iris Xe Graphics
	GPU2:
		apiVersion     = 1.3.280
		deviceType     = CPU
		deviceName     = llvmpipe (LLVM 17.0.6, 256 bits)
`
	require.Equal(t, []GpuInfo{
		{Index: 0, Name: "AMD Radeon RX 7900 XT"},
		{Index: 1, Name: "Intel(R) Iris Xe Graphics"},
	}, parseVulkanInfoOutput([]byte(summary)))

	// A device type is matched case-insensitively and independently of the
	// order the fields appear in.
	require.Empty(t, parseVulkanInfoOutput([]byte("GPU0:\n\tdeviceName = llvmpipe\n\tdeviceType = cpu\n")))
	// Blocks that never name a device are skipped, not fatal.
	require.Nil(t, parseVulkanInfoOutput([]byte("GPU0:\n\tdeviceType = DISCRETE_GPU\n")))
	// Broken lines between blocks are ignored; the next block still parses.
	require.Equal(t, []GpuInfo{{Index: 3, Name: "NVIDIA GeForce RTX 5090"}},
		parseVulkanInfoOutput([]byte("garbage\nGPUbroken:\nGPU3:\n\tdeviceName = NVIDIA GeForce RTX 5090\n")))
	require.Nil(t, parseVulkanInfoOutput([]byte("")))
}

// vulkanInfoStub writes a fake vulkaninfo that prints a fixed summary and
// returns its path, mirroring nvidiaSmiStub. The lines are printed without
// leading tabs, which the parser trims anyway.
func vulkanInfoStub(t *testing.T, rows ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vulkaninfo")
	if runtime.GOOS == "windows" {
		path += ".cmd"
	}
	var body string
	if runtime.GOOS == "windows" {
		body = "@echo off\r\n"
		for _, row := range rows {
			body += "echo " + row + "\r\n"
		}
	} else {
		body = "#!/bin/sh\n"
		for _, row := range rows {
			body += "echo " + strconv.Quote(row) + "\n"
		}
	}
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(path, 0o755))
	}
	return path
}

// TestSttVulkanGpusPointsAtStubbedTool drives the whole Vulkan enumeration
// through a stubbed command path: found GPUs are parsed with the CPU-type
// software renderer filtered out, and a missing tool reports no GPUs.
func TestSttVulkanGpusPointsAtStubbedTool(t *testing.T) {
	restore := SetVulkanInfoForTest(vulkanInfoStub(t,
		"GPU0:", "deviceType = DISCRETE_GPU", "deviceName = AMD Radeon RX 7900 XT",
		"GPU1:", "deviceType = CPU", "deviceName = llvmpipe (LLVM 17.0.6, 256 bits)"))
	defer restore()
	require.Equal(t, []GpuInfo{{Index: 0, Name: "AMD Radeon RX 7900 XT"}}, SttVulkanGpus())

	// A tool that is not on PATH means no GPUs, not an error.
	restore = SetVulkanInfoForTest("prime-no-such-vulkaninfo")
	defer restore()
	require.Nil(t, SttVulkanGpus())
}

// TestWhisperAcceleratorOptions pins the catalog composition: CPU always,
// CUDA only where the prebuilt runtime table has one and an NVIDIA GPU
// answers, Vulkan wherever a Vulkan GPU is reported, and every GPU tagged
// with the accelerator it belongs to.
func TestWhisperAcceleratorOptions(t *testing.T) {
	missing := "prime-no-such-tool"

	// Nothing on the machine: CPU is the only offer and no GPU is listed.
	restoreSmi := SetNvidiaSmiForTest(missing)
	restoreVk := SetVulkanInfoForTest(missing)
	defer func() { restoreSmi(); restoreVk() }()
	accelerators, gpus := WhisperAcceleratorOptions()
	require.Equal(t, []string{AcceleratorCPU}, accelerators)
	require.Empty(t, gpus)

	// Only a Vulkan GPU: vulkan becomes an offer on any platform.
	restoreVk = SetVulkanInfoForTest(vulkanInfoStub(t,
		"GPU0:", "deviceType = DISCRETE_GPU", "deviceName = AMD Radeon RX 9070 XT"))
	accelerators, gpus = WhisperAcceleratorOptions()
	require.Equal(t, []string{AcceleratorCPU, AcceleratorVulkan}, accelerators)
	require.Equal(t, []GpuInfo{{Index: 0, Name: "AMD Radeon RX 9070 XT", Kind: AcceleratorVulkan}}, gpus)

	// Only NVIDIA GPUs: the GPUs are listed for CUDA, but the accelerator
	// offer still follows the prebuilt runtime table.
	restoreVk = SetVulkanInfoForTest(missing)
	restoreSmi = SetNvidiaSmiForTest(nvidiaSmiStub(t, "0, NVIDIA GeForce RTX 5090"))
	accelerators, gpus = WhisperAcceleratorOptions()
	require.Equal(t, []GpuInfo{{Index: 0, Name: "NVIDIA GeForce RTX 5090", Kind: AcceleratorCUDA}}, gpus)
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		require.Equal(t, []string{AcceleratorCPU, AcceleratorCUDA}, accelerators)
	} else {
		require.Equal(t, []string{AcceleratorCPU}, accelerators)
	}

	// Both sources at once: NVIDIA first, Vulkan second, one entry each,
	// and no duplicate offers.
	restoreVk = SetVulkanInfoForTest(vulkanInfoStub(t,
		"GPU1:", "deviceType = DISCRETE_GPU", "deviceName = AMD Radeon RX 7900 XT"))
	accelerators, gpus = WhisperAcceleratorOptions()
	require.Len(t, gpus, 2)
	require.Equal(t, AcceleratorCUDA, gpus[0].Kind)
	require.Equal(t, AcceleratorVulkan, gpus[1].Kind)
	want := []string{AcceleratorCPU, AcceleratorVulkan}
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		want = []string{AcceleratorCPU, AcceleratorCUDA, AcceleratorVulkan}
	}
	require.Equal(t, want, accelerators)
}

// TestVulkanBuildToolsMissing checks the clear refusal when the build
// toolchain is absent, and that the refusal happens before the existing
// runtime is removed: a machine without the tools keeps its working CPU
// build.
func TestVulkanBuildToolsMissing(t *testing.T) {
	// Empty values mean the tools are not installed; the compiler entry is
	// ignored on Windows, which never requires it up front.
	restore := SetSttBuildToolsForTest(map[string]string{"cmake": "", "g++": "", "glslc": ""})
	defer restore()

	dataDir := t.TempDir()
	binDir := whisperBinDir(dataDir)
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "whisper-cli"), []byte("cpu-binary"), 0o755))

	err := ensureWhisperVulkanRuntime(context.Background(), dataDir, func(SetupProgress) {})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cmake")
	require.Contains(t, err.Error(), "glslc")
	if runtime.GOOS == "windows" {
		require.Contains(t, err.Error(), "Visual Studio")
	} else {
		require.Contains(t, err.Error(), "apt install")
	}
	require.FileExists(t, filepath.Join(binDir, "whisper-cli"), "the CPU build survives a refused Vulkan build")
	require.Equal(t, AcceleratorCPU, installedWhisperAccelerator(dataDir))
}

// vulkanCmakeStub writes a fake cmake that performs the two build steps a
// Vulkan install runs: configure creates the build directory, build drops a
// whisper-cli executable plus a shared library into build/bin. It appends
// every invocation to $PRIME_TEST_CMAKE_LOG so a test can pin the flags.
func vulkanCmakeStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cmake")
	if runtime.GOOS == "windows" {
		path += ".cmd"
		body := "@echo off\r\n" +
			"echo %* >> \"%PRIME_TEST_CMAKE_LOG%\"\r\n" +
			"if \"%~1\"==\"-S\" (\r\n" +
			"  mkdir \"%~3\" 2>nul\r\n" +
			"  exit /b 0\r\n" +
			")\r\n" +
			"if \"%~1\"==\"--build\" (\r\n" +
			"  mkdir \"%~2\\bin\" 2>nul\r\n" +
			"  echo vulkan-binary>\"%~2\\bin\\whisper-cli.exe\"\r\n" +
			"  echo vulkan-lib>\"%~2\\bin\\ggml-vulkan.dll\"\r\n" +
			"  exit /b 0\r\n" +
			")\r\n" +
			"exit /b 1\r\n"
		require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
		return path
	}
	body := `#!/bin/sh
echo "$@" >> "$PRIME_TEST_CMAKE_LOG"
if [ "$1" = "-S" ]; then
  mkdir -p "$3"
  exit 0
fi
if [ "$1" = "--install" ]; then
  exit 0
fi
if [ "$1" = "--build" ]; then
  mkdir -p "$2/bin"
  printf 'vulkan-binary' > "$2/bin/whisper-cli"
  printf 'vulkan-lib' > "$2/bin/libggml-vulkan.so"
  printf 'whisper-lib' > "$2/bin/libwhisper.so.1.9.5"
  ln -s libwhisper.so.1.9.5 "$2/bin/libwhisper.so.1"
  exit 0
fi
exit 1
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
	require.NoError(t, os.Chmod(path, 0o755))
	return path
}

// TestWhisperVulkanSourceBuild walks the Vulkan install path end to end with
// stubbed tools: the source tarball is downloaded and unpacked, both cmake
// steps run with the Vulkan flags, the build output (binary and shared
// libraries) lands in the bin directory with the marker set, the source tree
// is cleaned up, and a second call reuses the result without downloading.
// No real compiler runs: the cmake stub stands in for the actual build.
func TestWhisperVulkanSourceBuild(t *testing.T) {
	var sourceRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/source/") {
			sourceRequests.Add(1)
			tarGzWrite(t, w, map[string][]byte{
				"whisper.cpp-1.9.5/CMakeLists.txt":  []byte("cmake_minimum_required(VERSION 3.13)"),
				"whisper.cpp-1.9.5/src/whisper.cpp": []byte("source"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	setSttURLRewrites(map[string]string{whisperSourceURL: server.URL + "/source/"})
	t.Cleanup(func() { setSttURLRewrites(map[string]string{}) })

	logPath := filepath.Join(t.TempDir(), "cmake-args.log")
	t.Setenv("PRIME_TEST_CMAKE_LOG", logPath)
	restore := SetSttBuildToolsForTest(map[string]string{
		"cmake": vulkanCmakeStub(t),
		"g++":   "/usr/bin/g++",
		"glslc": "/usr/bin/glslc",
	})
	defer restore()
	// The host either has or lacks SDK headers; the test pins "present" so
	// the header download path stays out of this scenario.
	restoreProbe := SetVulkanHeadersProbeForTest(func() bool { return true })
	defer restoreProbe()

	dataDir := t.TempDir()
	var progresses []SetupProgress
	require.NoError(t, ensureWhisperVulkanRuntime(context.Background(), dataDir, func(progress SetupProgress) {
		progresses = append(progresses, progress)
	}))

	binDir := whisperBinDir(dataDir)
	require.NotEmpty(t, findWhisperBinary(binDir))
	// The shared libraries were copied next to the executable.
	entries, err := os.ReadDir(binDir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 2, "the build's shared libraries are copied with the binary")
	require.Equal(t, AcceleratorVulkan, installedWhisperAccelerator(dataDir))
	require.True(t, hasProgress(t, progresses, "build"))

	// Both cmake steps ran, and the Vulkan flag reached the configure step.
	log, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Contains(t, string(log), "-S")
	require.Contains(t, string(log), "-DGGML_VULKAN=1")
	require.Contains(t, string(log), "-DCMAKE_BUILD_TYPE=Release")
	require.Contains(t, string(log), "--build")

	// The extracted source tree did not survive the build.
	_, statErr := os.Stat(whisperBuildSrcDir(dataDir))
	require.True(t, os.IsNotExist(statErr), "the source tree is removed after the build")

	// Re-running the install reuses the built runtime: no new download.
	require.NoError(t, ensureWhisperVulkanRuntime(context.Background(), dataDir, func(SetupProgress) {}))
	require.Equal(t, int64(1), sourceRequests.Load())
}

// TestWhisperVulkanFetchesMissingHeaders covers machines without the SDK
// headers: the Khronos header tarballs are downloaded, unpacked and installed
// into the build prefix, and the cmake configure step receives the prefix so
// FindVulkan and FindSPIRV-Headers resolve.
func TestWhisperVulkanFetchesMissingHeaders(t *testing.T) {
	var headerRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/src/") {
			tarGzWrite(t, w, map[string][]byte{
				"whisper.cpp-1.9.5/CMakeLists.txt": []byte("cmake_minimum_required(VERSION 3.13)"),
			})
			return
		}
		if strings.Contains(r.URL.Path, "/vh/") {
			headerRequests.Add(1)
			tarGzWrite(t, w, map[string][]byte{
				vulkanHeadersDir + "/include/vulkan/vulkan.h": []byte("header"),
			})
			return
		}
		if strings.Contains(r.URL.Path, "/sh/") {
			headerRequests.Add(1)
			tarGzWrite(t, w, map[string][]byte{
				spirvHeadersDir + "/include/spirv/unified1/spirv.h": []byte("header"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	setSttURLRewrites(map[string]string{
		whisperSourceURL: server.URL + "/src/",
		vulkanHeadersURL: server.URL + "/vh/",
		spirvHeadersURL:  server.URL + "/sh/",
	})
	t.Cleanup(func() { setSttURLRewrites(map[string]string{}) })

	logPath := filepath.Join(t.TempDir(), "cmake-args.log")
	t.Setenv("PRIME_TEST_CMAKE_LOG", logPath)
	restore := SetSttBuildToolsForTest(map[string]string{
		"cmake": vulkanCmakeStub(t),
		"g++":   "/usr/bin/g++",
		"glslc": "/usr/bin/glslc",
	})
	defer restore()
	restoreProbe := SetVulkanHeadersProbeForTest(func() bool { return false })
	defer restoreProbe()

	dataDir := t.TempDir()
	require.NoError(t, ensureWhisperVulkanRuntime(context.Background(), dataDir, func(SetupProgress) {}))
	require.Equal(t, int64(2), headerRequests.Load(), "both header packages were fetched")
	log, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Contains(t, string(log), "-DCMAKE_PREFIX_PATH=")
	require.Contains(t, string(log), "sdk-prefix")
}
