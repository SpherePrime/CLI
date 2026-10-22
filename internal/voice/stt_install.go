package voice

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// sttHTTPClient downloads multi-hundred-megabyte models. No overall
// timeout: the install task's own context bounds the run, and a fixed
// client timeout would kill a slow but healthy multi-gigabyte transfer.
var sttHTTPClient = &http.Client{}

// downloadChunk is the read block size; one progress update flows out per
// chunk, which keeps the callback rate sane without stalling the UI.
const downloadChunk = 64 * 1024

// vulkanBuildTimeout bounds the whole cmake configure-and-build run for one
// Vulkan install. Compiling whisper.cpp takes minutes on a healthy machine;
// the generous ceiling exists so a hung compiler cannot spin forever, not
// because the build is expected to need it.
const vulkanBuildTimeout = 30 * time.Minute

// vulkanBuildJobs caps the parallel cmake jobs so a background build never
// saturates the machine the user is working on. Four jobs keep a desktop
// responsive while still building in minutes.
const vulkanBuildJobs = 4

// sttLookPath resolves a build tool through PATH. It is a package var so
// tests can stub the lookup instead of depending on the host toolchain.
var sttLookPath = exec.LookPath

// SetSttBuildToolsForTest replaces the build-tool lookup with a table for
// tests: name to resolved path. The table is authoritative — an empty value
// or a missing name means the tool is not installed. A nil map restores real
// PATH lookups. For tests outside this package.
func SetSttBuildToolsForTest(table map[string]string) func() {
	previous := sttLookPath
	if table == nil {
		sttLookPath = exec.LookPath
	} else {
		sttLookPath = func(name string) (string, error) {
			path, ok := table[name]
			if !ok || path == "" {
				return "", exec.ErrNotFound
			}
			return path, nil
		}
	}
	return func() { sttLookPath = previous }
}

// vulkanHeadersProbe reports whether the machine's Vulkan headers are in
// place. It is a package var so tests can pin the answer instead of
// depending on the host filesystem.
var vulkanHeadersProbe = vulkanHeadersPresent

// SetVulkanHeadersProbeForTest pins the Vulkan headers presence check. For
// tests outside this package; passing nil restores the real probe.
func SetVulkanHeadersProbeForTest(present func() bool) func() {
	previous := vulkanHeadersProbe
	if present == nil {
		vulkanHeadersProbe = vulkanHeadersPresent
	} else {
		vulkanHeadersProbe = present
	}
	return func() { vulkanHeadersProbe = previous }
}

// installWhisperModel brings in the whisper.cpp runtime when it is missing
// or built for another accelerator, and then the GGML model file, which is a
// plain download with no archive to unpack.
func installWhisperModel(ctx context.Context, dataDir, model, accelerator string, emit func(SetupProgress)) error {
	if _, ok := findSttModel(whisperModels, model); !ok {
		return fmt.Errorf("%w: whisper model %q", ErrSttUnknownModel, model)
	}
	// The runtime is ensured before the model check so switching the
	// accelerator takes effect even when the requested model is already on
	// disk; otherwise the switch would wait for the next model download.
	if err := ensureWhisperRuntime(ctx, dataDir, accelerator, emit); err != nil {
		return err
	}
	if fileExists(whisperModelPath(dataDir, model)) {
		emitReady(emit, "Whisper model "+model+" is already installed")
		return nil
	}

	target := whisperModelPath(dataDir, model)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	url := sttDownloadURL(whisperModelURLPrefix + model + ".bin")
	label := "Downloading " + model
	if err := downloadToFile(ctx, url, target, label, emit); err != nil {
		return err
	}
	emitReady(emit, "Whisper model "+model+" installed")
	return nil
}

// ensureWhisperRuntime brings the whisper.cpp command-line binary into the
// bin directory unless one is already there AND built for the accelerator
// the caller wants. Prebuilt accelerators (CPU, CUDA) are downloaded and
// unpacked; Vulkan has no prebuilt binaries at all, so that accelerator is
// compiled from source. The installed build is recorded in a marker file,
// because the variants unpack into the same directory and nothing else about
// the files tells them apart.
func ensureWhisperRuntime(ctx context.Context, dataDir, accelerator string, emit func(SetupProgress)) error {
	if accelerator == AcceleratorVulkan {
		return ensureWhisperVulkanRuntime(ctx, dataDir, emit)
	}
	url, variant, ok := whisperRuntime(accelerator)
	if !ok {
		return fmt.Errorf("whisper.cpp publishes no prebuilt macOS binary Prime can install; " +
			"install it manually (for example with brew install whisper-cpp) and the whisper engine will use it")
	}
	binDir := whisperBinDir(dataDir)
	if findWhisperBinary(binDir) != "" && installedWhisperAccelerator(dataDir) == variant {
		return nil
	}
	// A runtime built for another accelerator must not linger: its DLLs and
	// entry point would half-match the new archive, so the whole directory
	// goes before anything new lands in it.
	if err := os.RemoveAll(binDir); err != nil {
		return fmt.Errorf("replace whisper.cpp runtime: %w", err)
	}
	emit(SetupProgress{Phase: "runtime", Text: "Downloading whisper.cpp runtime..."})
	temp, err := os.CreateTemp("", "prime-whisper-runtime-*")
	if err != nil {
		return fmt.Errorf("stage runtime download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, sttDownloadURL(url), tempPath, "Downloading whisper.cpp runtime", emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "runtime", Text: "Extracting whisper.cpp runtime..."})
	if strings.HasSuffix(url, ".zip") {
		err = extractZip(tempPath, binDir)
	} else {
		err = extractTarGz(tempPath, binDir)
	}
	if err != nil {
		return fmt.Errorf("extract whisper.cpp runtime: %w", err)
	}
	// Windows-built archives often lose the executable bit, so it is set
	// explicitly on every platform; it is a no-op on Windows.
	if binary := findWhisperBinary(binDir); binary != "" && runtime.GOOS != "windows" {
		if err := os.Chmod(binary, 0o755); err != nil {
			return fmt.Errorf("mark whisper binary executable: %w", err)
		}
	}
	// The marker is written last: an interrupted install leaves no runtime
	// and no marker, and the next install starts from a clean slate.
	if err := writeWhisperAcceleratorMarker(dataDir, variant); err != nil {
		return fmt.Errorf("record whisper accelerator: %w", err)
	}
	return nil
}

// whisperBuildSrcDir is where the whisper.cpp source tarball is unpacked for
// a Vulkan build, inside the provider's own directory so a failed attempt
// leaves nothing outside the tree Remove knows about.
func whisperBuildSrcDir(dataDir string) string {
	return filepath.Join(sttRoot(dataDir), "whisper", "build-src")
}

// ensureWhisperVulkanRuntime builds whisper.cpp from source with the Vulkan
// backend, because whisper.cpp publishes no prebuilt Vulkan binaries and a
// source build with -DGGML_VULKAN=1 is the supported way to get GPU
// acceleration for any vendor (AMD, NVIDIA, Intel) on both Linux and
// Windows. The result lands in the same bin directory the prebuilt runtimes
// use, with the marker recording that this build speaks Vulkan.
func ensureWhisperVulkanRuntime(ctx context.Context, dataDir string, emit func(SetupProgress)) error {
	binDir := whisperBinDir(dataDir)
	// The installed runtime must be complete, not merely present: a build
	// from before soname links were copied passes the binary check but fails
	// at load time, so it is rebuilt instead of trusted.
	if findWhisperBinary(binDir) != "" && installedWhisperAccelerator(dataDir) == AcceleratorVulkan &&
		vulkanRuntimeComplete(binDir) {
		return nil
	}
	// The toolchain is verified before the old runtime is removed, so a
	// machine without the build tools keeps its working CPU build instead of
	// ending up with nothing and an error.
	if err := checkVulkanBuildTools(); err != nil {
		return err
	}
	// A runtime built for another accelerator must not linger: its DLLs and
	// entry point would half-match the new build, so the whole directory
	// goes before anything new lands in it.
	if err := os.RemoveAll(binDir); err != nil {
		return fmt.Errorf("replace whisper.cpp runtime: %w", err)
	}

	emit(SetupProgress{Phase: "runtime", Text: "Downloading whisper.cpp source..."})
	buildSrc := whisperBuildSrcDir(dataDir)
	// A previous failed attempt may have left a half-extracted tree behind.
	if err := os.RemoveAll(buildSrc); err != nil {
		return fmt.Errorf("clear whisper.cpp source tree: %w", err)
	}
	if err := os.MkdirAll(buildSrc, 0o755); err != nil {
		return err
	}
	// Source and build artifacts together weigh far more than the resulting
	// binaries, so the whole tree goes away once the build output is out.
	defer func() { _ = os.RemoveAll(buildSrc) }()

	temp, err := os.CreateTemp("", "prime-whisper-source-*")
	if err != nil {
		return fmt.Errorf("stage source download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, sttDownloadURL(whisperSourceURL), tempPath, "Downloading whisper.cpp source", emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "runtime", Text: "Extracting whisper.cpp source..."})
	if err := extractTarGz(tempPath, buildSrc); err != nil {
		return fmt.Errorf("extract whisper.cpp source: %w", err)
	}
	sourceDir := filepath.Join(buildSrc, whisperSourceDir)
	if !dirExists(sourceDir) {
		return fmt.Errorf("whisper.cpp source archive did not contain %s", whisperSourceDir)
	}

	emit(SetupProgress{Phase: "build", Text: "Building whisper.cpp for Vulkan, this can take a few minutes..."})
	if err := buildWhisperVulkan(ctx, sourceDir, emit); err != nil {
		return fmt.Errorf("build whisper.cpp for Vulkan: %w", err)
	}
	// Shared libraries the Vulkan backend loads live next to the executable
	// in build/bin, so every file there is collected, not just the binary.
	if err := copyBuiltWhisperBinaries(filepath.Join(sourceDir, "build", "bin"), binDir); err != nil {
		return fmt.Errorf("collect whisper.cpp build output: %w", err)
	}
	binary := findWhisperBinary(binDir)
	if binary == "" {
		return fmt.Errorf("the whisper.cpp Vulkan build produced no whisper-cli executable")
	}
	// Matches the prebuilt-runtime path: the executable bit is set
	// explicitly on unix, where a plain copy can lose it.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binary, 0o755); err != nil {
			return fmt.Errorf("mark whisper binary executable: %w", err)
		}
	}
	// The marker is written last, like every other runtime install: an
	// interrupted build leaves no runtime and no marker.
	if err := writeWhisperAcceleratorMarker(dataDir, AcceleratorVulkan); err != nil {
		return fmt.Errorf("record whisper accelerator: %w", err)
	}
	return nil
}

// vulkanCompilerTools names the C++ compilers a Vulkan build can use, in the
// order they are probed. On Windows plain PATH lookup is deliberately the
// only probe: cl.exe is usually absent from a user's PATH even with Visual
// Studio installed, and cmake's own Visual Studio generator discovery finds
// the compiler anyway, so a missing cl.exe here would wrongly block builds
// that would succeed.
func vulkanCompilerTools() []string {
	if runtime.GOOS == "windows" {
		return []string{"cl"}
	}
	return []string{"g++", "c++", "clang++"}
}

// checkVulkanBuildTools verifies that the machine has everything a
// GGML_VULKAN build needs before any download or destructive step runs, so
// the error a user sees is about installing a package, not about a compiler
// error half an hour into a build. On Windows the compiler is not required
// here: cmake finds Visual Studio on its own, and a genuinely broken setup
// surfaces in the streamed build log.
func checkVulkanBuildTools() error {
	var missing []string
	for _, tool := range []string{"cmake", "glslc"} {
		if _, err := sttLookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	compiler := ""
	for _, candidate := range vulkanCompilerTools() {
		if path, err := sttLookPath(candidate); err == nil {
			compiler = path
			break
		}
	}
	if compiler == "" {
		missing = append(missing, strings.Join(vulkanCompilerTools(), "/"))
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("building whisper.cpp for the Vulkan accelerator requires %s, which Prime did not find on this machine. %s",
		strings.Join(missing, ", "), vulkanToolchainHint())
}

// vulkanToolchainHint tells the user what to install for the missing tools,
// spelled out per platform because the package names differ everywhere.
func vulkanToolchainHint() string {
	if runtime.GOOS == "windows" {
		return "install CMake (cmake.org), Visual Studio Build Tools with the C++ workload, " +
			"and the Vulkan SDK (vulkan.lunarg.com), then retry"
	}
	return "install the Vulkan build toolchain: e.g. sudo apt install cmake build-essential " +
		"libvulkan-dev glslc (Debian/Ubuntu) or pacman -S cmake base-devel vulkan-devel " +
		"vulkan-headers shaderc (Arch), then retry"
}

// buildWhisperVulkan runs the cmake configure and build steps that produce a
// whisper.cpp with the Vulkan backend, sharing one time budget across both
// steps. Output is streamed into the install progress so a slow or failing
// build is visible instead of silent.
func buildWhisperVulkan(ctx context.Context, sourceDir string, emit func(SetupProgress)) error {
	buildCtx, cancel := context.WithTimeout(ctx, vulkanBuildTimeout)
	defer cancel()
	cmake, err := sttLookPath("cmake")
	if err != nil {
		// Unreachable in practice: checkVulkanBuildTools just verified the
		// same lookup, so a miss here is a race with a broken environment
		// worth naming rather than swallowing.
		return fmt.Errorf("resolve cmake: %w", err)
	}
	// The Khronos headers the Vulkan backend needs are only fetched when the
	// machine lacks them; with the SDK headers installed the system paths
	// win and the prefix stays out of the cmake command.
	prefix, err := prepareVulkanHeaders(ctx, filepath.Dir(sourceDir), emit)
	if err != nil {
		return err
	}
	configure := []string{
		cmake, "-S", sourceDir, "-B", filepath.Join(sourceDir, "build"),
		"-DGGML_VULKAN=1", "-DCMAKE_BUILD_TYPE=Release",
	}
	if prefix != "" {
		configure = append(configure, "-DCMAKE_PREFIX_PATH="+prefix)
	}
	buildDir := filepath.Join(sourceDir, "build")
	steps := [][]string{
		configure,
		// The job count is capped because the build runs in the background of
		// a desktop session: every core belongs to the compiler only on a
		// build server, not while someone is using the machine.
		{cmake, "--build", buildDir, "--config", "Release", "-j", fmt.Sprintf("%d", min(vulkanBuildJobs, runtime.NumCPU()))},
	}
	for _, step := range steps {
		if err := runBuildStep(buildCtx, sourceDir, step, emit); err != nil {
			return err
		}
	}
	return nil
}

// prepareVulkanHeaders makes sure cmake can find the Vulkan and SPIR-V
// headers ggml-vulkan requires. Machines with the SDK headers installed are
// detected and left alone; the rest get the Khronos header-only tarballs
// downloaded and installed into a local prefix next to the whisper source,
// so no system package installation is ever required.
func prepareVulkanHeaders(ctx context.Context, buildSrc string, emit func(SetupProgress)) (string, error) {
	if vulkanHeadersProbe() {
		return "", nil
	}
	prefix := filepath.Join(buildSrc, "sdk-prefix")
	headers := []struct{ name, url, dir string }{
		{"Vulkan", vulkanHeadersURL, vulkanHeadersDir},
		{"SPIR-V", spirvHeadersURL, spirvHeadersDir},
	}
	for _, entry := range headers {
		temp, err := os.CreateTemp("", "prime-vulkan-headers-*")
		if err != nil {
			return "", fmt.Errorf("stage %s headers download: %w", entry.name, err)
		}
		tempPath := temp.Name()
		stageErr := func() error {
			defer func() { _ = os.Remove(tempPath) }()
			if err := temp.Close(); err != nil {
				return err
			}
			if err := downloadToFile(ctx, sttDownloadURL(entry.url), tempPath, "Downloading "+entry.name+" headers", emit); err != nil {
				return err
			}
			extractDir := filepath.Join(buildSrc, entry.name+"-headers")
			if err := os.RemoveAll(extractDir); err != nil {
				return err
			}
			if err := extractTarGz(tempPath, extractDir); err != nil {
				return fmt.Errorf("extract %s headers: %w", entry.name, err)
			}
			source := filepath.Join(extractDir, entry.dir)
			if !dirExists(source) {
				return fmt.Errorf("the %s headers archive did not contain %s", entry.name, entry.dir)
			}
			cmake, err := sttLookPath("cmake")
			if err != nil {
				return err
			}
			build := filepath.Join(extractDir, "build")
			for _, step := range [][]string{
				{cmake, "-S", source, "-B", build, "-DCMAKE_INSTALL_PREFIX=" + prefix},
				{cmake, "--install", build},
			} {
				if err := runBuildStep(ctx, extractDir, step, emit); err != nil {
					return fmt.Errorf("install %s headers: %w", entry.name, err)
				}
			}
			return nil
		}()
		if stageErr != nil {
			return "", stageErr
		}
	}
	return prefix, nil
}

// vulkanHeadersPresent reports whether the machine already ships the Vulkan
// headers cmake's FindVulkan needs, either from the distribution packages or
// from an installed Vulkan SDK on Windows.
func vulkanHeadersPresent() bool {
	paths := []string{
		"/usr/include/vulkan/vulkan.h",
		"/usr/local/include/vulkan/vulkan.h",
	}
	if runtime.GOOS == "windows" {
		if sdk := os.Getenv("VULKAN_SDK"); sdk != "" {
			paths = []string{filepath.Join(sdk, "Include", "vulkan", "vulkan.h")}
		}
	}
	for _, path := range paths {
		if fileExists(path) {
			return true
		}
	}
	return false
}

// runBuildStep runs one build command from dir, forwarding its combined
// output lines as build progress. It mirrors runInstaller: stdin is closed
// because Prime owns the terminal, and a deadline overrun is reported as
// the timeout it was rather than a bare "killed".
func runBuildStep(ctx context.Context, dir string, command []string, emit func(SetupProgress)) error {
	// The build runs below normal priority: minutes of compiling must not
	// fight the desktop the user is working on.
	command = lowPriorityCommand(command)
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.SysProcAttr = lowPrioritySysAttr()
	// Build tools must never read the terminal, which Prime owns.
	cmd.Stdin = nil
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}
	var tail []string
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			emit(SetupProgress{Phase: "build", Text: "  " + line})
			// A failure reported as a bare exit status is useless: the last
			// output lines carry the actual compiler or cmake error.
			tail = append(tail, line)
			if len(tail) > 25 {
				tail = tail[len(tail)-25:]
			}
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("whisper.cpp build timed out after %s", vulkanBuildTimeout)
	}
	if waitErr != nil && len(tail) > 0 {
		return fmt.Errorf("%w\nlast build output:\n%s", waitErr, strings.Join(tail, "\n"))
	}
	return waitErr
}

// copyBuiltWhisperBinaries copies every regular file of the build's bin
// directory into the installed bin directory: the whisper-cli executable and
// the shared libraries ggml-vulkan loads at run time sit side by side there,
// and skipping subdirectories keeps the flat layout the prebuilt archives
// establish.
func copyBuiltWhisperBinaries(buildBinDir, binDir string) error {
	entries, err := os.ReadDir(buildBinDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		// Shared libraries are linked through their soname, which is a
		// symlink pointing at the versioned file beside it, so symlinks are
		// recreated rather than skipped: without them the dynamic loader
		// reports a missing libwhisper.so.1 at run time.
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(filepath.Join(buildBinDir, entry.Name()))
			if err != nil {
				return err
			}
			target := filepath.Join(binDir, entry.Name())
			_ = os.Remove(target)
			if err := os.Symlink(link, target); err != nil && !os.IsExist(err) {
				return err
			}
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		source, err := os.Open(filepath.Join(buildBinDir, entry.Name()))
		if err != nil {
			return err
		}
		err = writeFileWithMode(filepath.Join(binDir, entry.Name()), source, info.Mode().Perm())
		_ = source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// vulkanRuntimeComplete reports whether an installed Vulkan runtime carries
// the soname links its binary needs, so an install from before symlink
// copying existed is treated as missing and rebuilt instead of failing at
// transcription time.
func vulkanRuntimeComplete(binDir string) bool {
	if runtime.GOOS == "windows" {
		return findWhisperBinary(binDir) != ""
	}
	for _, soname := range []string{"libwhisper.so.1", "libwhisper.1.dylib"} {
		if fileExists(filepath.Join(binDir, soname)) {
			return true
		}
	}
	return false
}

// installVoskModel brings in the libvosk shared library when it is missing
// and then the model zip, which extracts into its own folder.
func installVoskModel(ctx context.Context, dataDir, model string, emit func(SetupProgress)) error {
	if _, ok := findSttModel(voskModels, model); !ok {
		return fmt.Errorf("%w: vosk model %q", ErrSttUnknownModel, model)
	}
	if dirExists(voskModelDir(dataDir, model)) {
		emitReady(emit, "Vosk model "+model+" is already installed")
		return nil
	}
	if err := ensureVoskRuntime(ctx, dataDir, emit); err != nil {
		return err
	}

	url := sttDownloadURL(voskModelURLPrefix + model + ".zip")
	emit(SetupProgress{Phase: "download", Text: "Downloading " + model + "...", Total: voskModelSize(model)})
	temp, err := os.CreateTemp("", "prime-vosk-model-*")
	if err != nil {
		return fmt.Errorf("stage model download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, url, tempPath, "Downloading "+model, emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "extract", Text: "Extracting " + model + "..."})
	// The zip ships a top-level folder named after the model, so extracting
	// into models/ produces models/<model>/... directly.
	if err := extractZip(tempPath, filepath.Dir(voskModelDir(dataDir, model))); err != nil {
		return fmt.Errorf("extract vosk model: %w", err)
	}
	emitReady(emit, "Vosk model "+model+" installed")
	return nil
}

// ensureVoskRuntime downloads and unpacks the libvosk shared library
// unless it is already in the lib directory.
func ensureVoskRuntime(ctx context.Context, dataDir string, emit func(SetupProgress)) error {
	if fileExists(voskLibPath(dataDir)) {
		return nil
	}
	url, ok := voskRuntime()
	if !ok {
		return fmt.Errorf("vosk publishes no prebuilt library for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	emit(SetupProgress{Phase: "runtime", Text: "Downloading Vosk runtime..."})
	temp, err := os.CreateTemp("", "prime-vosk-runtime-*")
	if err != nil {
		return fmt.Errorf("stage runtime download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, sttDownloadURL(url), tempPath, "Downloading Vosk runtime", emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "runtime", Text: "Extracting Vosk runtime..."})
	if err := extractZip(tempPath, voskLibDir(dataDir)); err != nil {
		return fmt.Errorf("extract vosk runtime: %w", err)
	}
	// The release zips ship the library inside a top-level folder named
	// after the release, so the file is moved up next to anything else the
	// archive carried before the presence check runs.
	if err := relocateFile(voskLibDir(dataDir), filepath.Base(voskLibPath(dataDir))); err != nil {
		return fmt.Errorf("relocate vosk runtime: %w", err)
	}
	if !fileExists(voskLibPath(dataDir)) {
		return fmt.Errorf("vosk runtime archive contained no %s", filepath.Base(voskLibPath(dataDir)))
	}
	return nil
}

// relocateFile lifts a nested file up to the root of dir so a platform
// check that expects a fixed layout does not care how the release packaged
// it. Only the first match moves; release zips carry exactly one.
func relocateFile(dir, name string) error {
	found := ""
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || found != "" {
			return nil //nolint:nilerr // a missing directory just means "nothing to move"
		}
		if entry.Name() == name && filepath.Dir(path) != dir {
			found = path
		}
		return nil
	})
	if err != nil || found == "" {
		return err
	}
	source, err := os.Open(found)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if err := writeFileWithMode(filepath.Join(dir, name), source, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Remove(found)
}

// findSttModel locates a catalog entry by id.
func findSttModel(models []SttModel, id string) (SttModel, bool) {
	for _, model := range models {
		if model.ID == id {
			return model, true
		}
	}
	return SttModel{}, false
}

// voskModelSize reads the advertised size out of the catalog so the
// progress bar can start from a sane total before the server answers.
func voskModelSize(model string) int64 {
	if entry, ok := findSttModel(voskModels, model); ok {
		return entry.SizeBytes
	}
	return 0
}

// emitReady closes an install run with the success line the menu shows.
func emitReady(emit func(SetupProgress), text string) {
	emit(SetupProgress{Phase: "ready", Text: text, Done: 1, Total: 1})
}

// progressInterval bounds how often a download reports progress. A chunk
// arrives every few milliseconds at broadband speed, and reporting each one
// would spin the task bookkeeping thousands of times a second for no visual
// gain: the settings screen polls progress twice a second anyway.
const progressInterval = 250 * time.Millisecond

// downloadToFile streams url into dest, reporting byte progress under the
// given label. The file is written under its final name in place: the
// install task holds the only reference, and a failed download leaves a
// partial file that SttIsInstalled's checks will not mistake for a model
// because whisper models are validated by size on load and vosk models by
// their extracted directory, which only appears after a successful unpack.
func downloadToFile(ctx context.Context, url, dest, label string, emit func(SetupProgress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	res, err := sttHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", label, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s answered %s", label, url, res.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	total := res.ContentLength
	done := int64(0)
	lastReport := time.Time{}
	buf := make([]byte, downloadChunk)
	for {
		read, readErr := res.Body.Read(buf)
		if read > 0 {
			if _, writeErr := out.Write(buf[:read]); writeErr != nil {
				_ = out.Close()
				return fmt.Errorf("download %s: %w", label, writeErr)
			}
			done += int64(read)
			if now := time.Now(); now.Sub(lastReport) >= progressInterval {
				lastReport = now
				emit(SetupProgress{Phase: "download", Text: label, Done: done, Total: total})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = out.Close()
			return fmt.Errorf("download %s: %w", label, readErr)
		}
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("download %s: %w", label, err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// safeExtractTarget resolves an archive entry name against dest and refuses
// anything that would escape it: absolute paths and ".." elements are the
// classic malicious-archive trick and are rejected rather than cleaned away.
func safeExtractTarget(dest, name string) (string, error) {
	if strings.ContainsRune(name, 0) || filepath.IsAbs(name) || strings.HasPrefix(name, "/") ||
		strings.Contains(name, `\`) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	clean := filepath.Clean(filepath.Join(dest, filepath.FromSlash(name)))
	if clean != dest && !strings.HasPrefix(clean, dest+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	return clean, nil
}

// extractZip unpacks a zip archive into dest, creating directories as the
// entries demand. Entries keep their stored permissions where they are set.
func extractZip(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, entry := range reader.File {
		target, err := safeExtractTarget(dest, entry.Name)
		if err != nil {
			return err
		}
		if err := extractZipEntry(entry, target); err != nil {
			return err
		}
	}
	return nil
}

// extractZipEntry writes one zip entry, which is either a directory or a
// regular file; other types (symlinks, in particular) are skipped because
// an archive must not plant links outside its own tree.
func extractZipEntry(entry *zip.File, target string) error {
	if entry.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if !entry.FileInfo().Mode().IsRegular() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	return writeFileWithMode(target, source, entry.FileInfo().Mode())
}

// extractTarGz unpacks a gzipped tarball into dest.
func extractTarGz(archivePath, dest string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeExtractTarget(dest, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Only regular files are extracted: links and devices have no
			// place in a model directory and symlinks could point anywhere.
			if err := writeFileWithMode(target, reader, os.FileMode(header.Mode)&0o777); err != nil {
				return err
			}
		}
	}
}

// writeFileWithMode creates target and copies source into it.
func writeFileWithMode(target string, source io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, source); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
