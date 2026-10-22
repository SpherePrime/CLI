package proto

// SttCatalog is the speech-to-text state of one workspace: the engine
// currently configured and every provider with its downloadable models.
type SttCatalog struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
	// Language is the configured dictation language; empty means the engine
	// detects it.
	Language  string        `json:"language"`
	Providers []SttProvider `json:"providers"`
}

// SttProvider is one transcription engine the desktop can offer.
type SttProvider struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Available   bool       `json:"available"`
	Models      []SttModel `json:"models"`
	// Accelerator is the compute device configured for whisper: "cpu",
	// "cuda" or "vulkan". Every other provider reports empty, because it has
	// no choice.
	Accelerator string `json:"accelerator,omitempty"`
	// GPUs lists the GPUs the machine reports for whisper, each tagged with
	// the accelerator it belongs to; whisper only, empty for the rest.
	GPUs []Gpu `json:"gpus"`
	// Accelerators lists the compute devices whisper can be installed for
	// here; whisper only, empty for the rest.
	Accelerators []string `json:"accelerators"`
}

// Gpu is one GPU the machine exposes to whisper.cpp.
type Gpu struct {
	// Index is the device index stored in options.voice.gpu_device. The
	// numbering is per kind: a CUDA index comes from nvidia-smi, a Vulkan
	// index from vulkaninfo.
	Index int `json:"index"`
	// Name is the human-readable adapter name.
	Name string `json:"name"`
	// Kind names the acceleration API the GPU was found through: "cuda"
	// (NVIDIA via nvidia-smi) or "vulkan" (any vendor via vulkaninfo).
	Kind string `json:"kind"`
}

// SttModel is one downloadable model of a provider.
type SttModel struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Language    string `json:"language"`
	SizeBytes   int64  `json:"size_bytes"`
	Recommended bool   `json:"recommended"`
	Installed   bool   `json:"installed"`
	Active      bool   `json:"active"`
}

// SttModelRequest names a model to install, remove or select. The select
// call also carries the settings that belong to the engine: the dictation
// language and, for whisper, the compute device.
type SttModelRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Language pins the dictation language; empty means auto-detect.
	Language string `json:"language,omitempty"`
	// Accelerator selects the whisper compute device: "cpu", "cuda" or
	// "vulkan".
	Accelerator string `json:"accelerator,omitempty"`
	// GPUDevice is the device index used with a GPU accelerator: the CUDA
	// device for cuda, the Vulkan device for vulkan.
	GPUDevice string `json:"gpu_device,omitempty"`
}

// SttSelection echoes the engine and model after a select.
type SttSelection struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
}

// Started confirms an asynchronous operation was accepted.
type Started struct {
	Started bool `json:"started"`
}
