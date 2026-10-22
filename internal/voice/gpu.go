package voice

import (
	"context"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GPU enumeration for the whisper engine. Prime never touches the graphics
// drivers itself: it asks nvidia-smi for the CUDA-capable NVIDIA GPUs and
// vulkaninfo for the Vulkan-capable devices, so the desktop can offer the
// accelerators the machine can actually run.

// GpuInfo is one GPU the machine reports for local inference.
type GpuInfo struct {
	// Index is the device index, the value options.voice.gpu_device stores.
	// The numbering is per acceleration API: a CUDA index comes from
	// nvidia-smi, a Vulkan index from vulkaninfo.
	Index int
	// Name is the human-readable adapter name, e.g. "NVIDIA GeForce RTX 3070".
	Name string
	// Kind names the acceleration API the device is reachable through:
	// AcceleratorCUDA or AcceleratorVulkan. It is set by
	// WhisperAcceleratorOptions, which knows which enumeration produced the
	// entry; the individual enumerators leave it empty.
	Kind string
}

// nvidiaSmiCommand is the tool NVIDIA GPU enumeration runs, resolved through
// PATH the same way ffmpeg and friends are. It is a package var so tests can
// point it at a stub.
var nvidiaSmiCommand = "nvidia-smi"

// nvidiaSmiTimeout bounds one enumeration. The query is a metadata read that
// answers in milliseconds on a healthy driver; a hung driver must not stall
// the settings screen, so a slow answer counts as no GPUs.
const nvidiaSmiTimeout = 5 * time.Second

// gpuCacheTTL bounds how long one enumeration result is reused. The settings
// screen polls the catalog every second and a half, and spawning nvidia-smi
// and vulkaninfo at that rate is wasteful process churn; a short cache keeps
// the lists fresh for hot-plugging without the constant tool runs.
const gpuCacheTTL = time.Minute

var (
	gpuCacheMu     sync.Mutex
	nvidiaGpuCache []GpuInfo
	nvidiaGpuAt    time.Time
	vulkanGpuCache []GpuInfo
	vulkanGpuAt    time.Time
)

// cachedGpus returns the cached list when it is younger than the TTL.
func cachedGpus(list *[]GpuInfo, at *time.Time) ([]GpuInfo, bool) {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()
	if time.Since(*at) < gpuCacheTTL {
		return *list, true
	}
	return nil, false
}

// storeGpus saves an enumeration result in the cache.
func storeGpus(list *[]GpuInfo, at *time.Time, gpus []GpuInfo) {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()
	*list = gpus
	*at = time.Now()
}

// resetGpuCache drops both cached enumerations, used by the test hooks so a
// stubbed tool is actually consulted.
func resetGpuCache() {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()
	nvidiaGpuCache, nvidiaGpuAt = nil, time.Time{}
	vulkanGpuCache, vulkanGpuAt = nil, time.Time{}
}

// SttNvidiaGpus lists the NVIDIA GPUs nvidia-smi reports, in driver order.
// An empty slice means the tool is missing, failed, or saw no GPU, which for
// the catalog's purposes are all the same state: no CUDA to offer. Results
// are cached briefly, because the settings screen polls the catalog.
func SttNvidiaGpus() []GpuInfo {
	if gpus, ok := cachedGpus(&nvidiaGpuCache, &nvidiaGpuAt); ok {
		return gpus
	}
	path, err := exec.LookPath(nvidiaSmiCommand)
	if err != nil {
		storeGpus(&nvidiaGpuCache, &nvidiaGpuAt, nil)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), nvidiaSmiTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--query-gpu=index,name", "--format=csv,noheader").Output()
	if err != nil {
		storeGpus(&nvidiaGpuCache, &nvidiaGpuAt, nil)
		return nil
	}
	gpus := parseNvidiaSmiOutput(out)
	storeGpus(&nvidiaGpuCache, &nvidiaGpuAt, gpus)
	return gpus
}

// parseNvidiaSmiOutput reads rows like "0, NVIDIA GeForce RTX 3070". Rows
// that do not parse are skipped rather than fatal: one broken line must not
// hide the GPUs the driver did report.
func parseNvidiaSmiOutput(out []byte) []GpuInfo {
	var gpus []GpuInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, ",", 2)
		if len(fields) != 2 {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil || index < 0 {
			continue
		}
		gpus = append(gpus, GpuInfo{Index: index, Name: strings.TrimSpace(fields[1])})
	}
	return gpus
}

// SetNvidiaSmiForTest points GPU enumeration at a stub command. An empty
// name restores nvidia-smi. For tests outside this package.
func SetNvidiaSmiForTest(command string) func() {
	previous := nvidiaSmiCommand
	nvidiaSmiCommand = command
	resetGpuCache()
	return func() {
		nvidiaSmiCommand = previous
		resetGpuCache()
	}
}

// vulkanInfoCommand is the tool Vulkan GPU enumeration runs, resolved through
// PATH the same way nvidia-smi is. It is a package var so tests can point it
// at a stub.
var vulkanInfoCommand = "vulkaninfo"

// vulkanInfoTimeout bounds one enumeration. The summary run loads the Vulkan
// loader and queries every driver, which takes longer than nvidia-smi but is
// still a metadata read; a hung loader must not stall the settings screen,
// so a slow answer counts as no GPUs.
const vulkanInfoTimeout = 10 * time.Second

// SttVulkanGpus lists the Vulkan-capable GPUs vulkaninfo --summary reports,
// in loader order. An empty slice means the tool is missing, failed, or saw
// no GPU, which for the catalog's purposes are all the same state: no Vulkan
// to offer. Software rendering is filtered out by the parser, because
// llvmpipe reports itself as a device but cannot accelerate anything.
func SttVulkanGpus() []GpuInfo {
	if gpus, ok := cachedGpus(&vulkanGpuCache, &vulkanGpuAt); ok {
		return gpus
	}
	path, err := exec.LookPath(vulkanInfoCommand)
	if err != nil {
		storeGpus(&vulkanGpuCache, &vulkanGpuAt, nil)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), vulkanInfoTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--summary").Output()
	if err != nil {
		storeGpus(&vulkanGpuCache, &vulkanGpuAt, nil)
		return nil
	}
	gpus := parseVulkanInfoOutput(out)
	storeGpus(&vulkanGpuCache, &vulkanGpuAt, gpus)
	return gpus
}

// gpuHeaderRE matches the device headers of `vulkaninfo --summary` output,
// lines like "GPU0:" that start one device block.
var gpuHeaderRE = regexp.MustCompile(`^GPU(\d+):$`)

// gpuFieldRE matches the "name = value" device fields inside a block.
var gpuFieldRE = regexp.MustCompile(`^(deviceName|deviceType)\s*=\s*(.+)$`)

// parseVulkanInfoOutput reads the Devices block of `vulkaninfo --summary`.
// A "GPU<n>:" line starts a device, the following deviceName line names it,
// and the deviceType line can veto it: the llvmpipe software renderer shows
// up as a device whose type is CPU, and offering it as an accelerator would
// end in a slower whisper run than the CPU build alone. Blocks that never
// name a device are skipped rather than fatal, because one malformed entry
// must not hide the GPUs the loader did report.
func parseVulkanInfoOutput(out []byte) []GpuInfo {
	index, name, deviceType := -1, "", ""
	var gpus []GpuInfo
	flush := func() {
		if index >= 0 && name != "" && !strings.EqualFold(strings.TrimSpace(deviceType), "cpu") {
			gpus = append(gpus, GpuInfo{Index: index, Name: name})
		}
		index, name, deviceType = -1, "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if header := gpuHeaderRE.FindStringSubmatch(line); header != nil {
			flush()
			if parsed, err := strconv.Atoi(header[1]); err == nil {
				index = parsed
			}
			continue
		}
		if index < 0 {
			continue
		}
		field := gpuFieldRE.FindStringSubmatch(line)
		if field == nil {
			continue
		}
		switch field[1] {
		case "deviceName":
			name = strings.TrimSpace(field[2])
		case "deviceType":
			deviceType = strings.TrimSpace(field[2])
		}
	}
	flush()
	return gpus
}

// SetVulkanInfoForTest points Vulkan GPU enumeration at a stub command. An
// empty name restores vulkaninfo. For tests outside this package.
func SetVulkanInfoForTest(command string) func() {
	previous := vulkanInfoCommand
	vulkanInfoCommand = command
	resetGpuCache()
	return func() {
		vulkanInfoCommand = previous
		resetGpuCache()
	}
}

// WhisperAcceleratorOptions composes what the catalog needs for the whisper
// engine in one pass: the accelerators this machine can actually offer, and
// the GPUs behind them, each tagged with the accelerator it belongs to. Both
// enumerations run exactly once here, so a catalog request never pays for a
// duplicate nvidia-smi or vulkaninfo round trip.
//
// The CPU build always heads the list. CUDA joins only where a prebuilt CUDA
// runtime is published (Windows x64) AND an NVIDIA GPU answers nvidia-smi.
// Vulkan joins wherever at least one Vulkan-capable GPU is reported, on any
// platform, because that runtime is built from source with whatever
// toolchain the machine has.
func WhisperAcceleratorOptions() (accelerators []string, gpus []GpuInfo) {
	nvidia := SttNvidiaGpus()
	vulkan := SttVulkanGpus()
	gpus = make([]GpuInfo, 0, len(nvidia)+len(vulkan))
	for _, gpu := range nvidia {
		gpu.Kind = AcceleratorCUDA
		gpus = append(gpus, gpu)
	}
	for _, gpu := range vulkan {
		gpu.Kind = AcceleratorVulkan
		gpus = append(gpus, gpu)
	}
	accelerators = []string{AcceleratorCPU}
	if slices.Contains(WhisperAccelerators(), AcceleratorCUDA) && len(nvidia) > 0 {
		accelerators = append(accelerators, AcceleratorCUDA)
	}
	if len(vulkan) > 0 {
		accelerators = append(accelerators, AcceleratorVulkan)
	}
	return accelerators, gpus
}
