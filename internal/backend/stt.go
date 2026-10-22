package backend

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/voice"
)

// Local STT model management. The desktop needs a catalog with install
// state, background installs with progress, and a config write to switch
// engines. Install tasks ride the same plumbing the plugin installs use
// (plugins.Task plus ws.pluginTasks), keyed "stt:<provider>:<model>" so the
// plugin task list and the STT task list can each show their own slice.

var (
	// ErrSttTaskRunning reports a duplicate install of the same model.
	ErrSttTaskRunning = errors.New("an install for this model is already running")
	// ErrSttUnknownProvider reports an engine the desktop may not pick.
	ErrSttUnknownProvider = errors.New("unknown STT engine")
)

// sttRemoveWait bounds how long removing a model waits for a cancelled
// install to unwind before the files are deleted anyway.
const sttRemoveWait = 10 * time.Second

// sttTaskKey is the pluginTasks key under which an STT install travels.
func sttTaskKey(provider, model string) string {
	return "stt:" + provider + ":" + model
}

// isSttTaskKey reports whether a pluginTasks entry belongs to STT.
func isSttTaskKey(name string) bool {
	return strings.HasPrefix(name, "stt:")
}

// SttCatalog returns the configured engine plus every provider with its
// models and their install state, along with the accelerator and GPU
// information the desktop needs to offer CUDA and Vulkan for whisper.
func (b *Backend) SttCatalog(workspaceID string) (proto.SttCatalog, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.SttCatalog{}, err
	}
	cfg := ws.Cfg.Config()
	options := cfg.Options.Voice
	engine, model, language, accelerator := "", "", "", ""
	if options != nil {
		// Lowercased like voice.SettingsFrom normalizes it, so the catalog
		// shows what dictation would actually run.
		engine = strings.ToLower(strings.TrimSpace(options.Engine))
		model = strings.TrimSpace(options.Model)
		language = strings.TrimSpace(options.Language)
		accelerator = sttConfiguredAccelerator(options.Accelerator)
	}
	// Accelerators and GPUs are enumerated once per request: whisper is the
	// only engine that can use them, and the answers decide which compute
	// devices are offers at all.
	accelerators, gpus := voice.WhisperAcceleratorOptions()

	providers := voice.SttCatalog(cfg.Options.DataDirectory)
	result := proto.SttCatalog{
		Engine:    engine,
		Model:     model,
		Language:  language,
		Providers: make([]proto.SttProvider, 0, len(providers)),
	}
	for _, provider := range providers {
		models := make([]proto.SttModel, 0, len(provider.Models))
		for _, entry := range provider.Models {
			models = append(models, proto.SttModel{
				ID:          entry.ID,
				Label:       entry.Label,
				Language:    entry.Language,
				SizeBytes:   entry.SizeBytes,
				Recommended: entry.Recommended,
				Installed:   entry.Installed,
				Active:      sttModelActive(provider.ID, entry, engine, model),
			})
		}
		entry := proto.SttProvider{
			ID:          provider.ID,
			Title:       provider.Title,
			Description: provider.Description,
			Available:   provider.Available,
			Models:      models,
			GPUs:        []proto.Gpu{},
		}
		if provider.ID == voice.SttWhisper {
			entry.Accelerator = accelerator
			entry.GPUs = sttGpusToProto(gpus)
			entry.Accelerators = accelerators
		}
		result.Providers = append(result.Providers, entry)
	}
	return result, nil
}

// sttConfiguredAccelerator returns the accelerator the config names, when it
// is one the whisper engine accepts; anything else reports as unconfigured.
func sttConfiguredAccelerator(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case voice.AcceleratorCPU, voice.AcceleratorCUDA, voice.AcceleratorVulkan:
		return normalized
	}
	return ""
}

// sttGpusToProto copies the GPU list into the wire shape, carrying the kind
// of accelerator each device belongs to.
func sttGpusToProto(gpus []voice.GpuInfo) []proto.Gpu {
	result := make([]proto.Gpu, 0, len(gpus))
	for _, gpu := range gpus {
		result = append(result, proto.Gpu{Index: gpu.Index, Name: gpu.Name, Kind: gpu.Kind})
	}
	return result
}

// sttModelActive marks the model dictation would use today. An engine
// configured without a model falls back to its recommended installed one,
// which is what the transcriber would have to be told anyway.
func sttModelActive(providerID string, entry voice.SttModelInfo, engine, model string) bool {
	if model != "" {
		return providerID == engine && entry.ID == model
	}
	return providerID == engine && entry.Recommended && entry.Installed
}

// StartSttInstall runs voice.SttInstall in the background, reporting
// progress through the workspace task store. A duplicate install of the
// same model is refused rather than queued twice.
func (b *Backend) StartSttInstall(workspaceID, provider, model string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	cfg := ws.Cfg.Config()
	dataDir := cfg.Options.DataDirectory
	// The accelerator is captured when the task starts: a config change
	// mid-install must not re-target a download that already began.
	accelerator := ""
	if cfg.Options.Voice != nil {
		accelerator = sttConfiguredAccelerator(cfg.Options.Voice.Accelerator)
	}
	key := sttTaskKey(provider, model)

	ws.pluginMu.Lock()
	defer ws.pluginMu.Unlock()
	if task := ws.pluginTasks[key]; task != nil && task.Snapshot().State == plugins.TaskRunning {
		return ErrSttTaskRunning
	}
	plugin := plugins.Plugin{
		Name: key,
		Install: func(ctx context.Context, _ voice.Settings, report func(plugins.Progress)) error {
			return voice.SttInstall(ctx, dataDir, provider, model, accelerator, func(progress voice.SetupProgress) {
				report(plugins.Progress{Phase: progress.Phase, Text: progress.Text, Done: progress.Done, Total: progress.Total})
			})
		},
	}
	if ws.pluginTasks == nil {
		ws.pluginTasks = make(map[string]*plugins.Task)
	}
	ws.pluginTasks[key] = plugins.InstallAsync(plugin, voice.Settings{})
	return nil
}

// RemoveSttModel cancels a running install of the model, waits briefly for
// it to unwind, then deletes the files. Removing a model that is not
// installed is not an error: the desktop wants the entry gone either way.
func (b *Backend) RemoveSttModel(workspaceID, provider, model string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	key := sttTaskKey(provider, model)

	ws.pluginMu.Lock()
	task := ws.pluginTasks[key]
	running := task != nil && task.Snapshot().State == plugins.TaskRunning
	ws.pluginMu.Unlock()
	if running {
		task.Cancel()
		select {
		case <-task.Done():
		case <-time.After(sttRemoveWait):
			slog.Warn("STT install did not stop after cancel", "provider", provider, "model", model)
		}
	}

	dataDir := ws.Cfg.Config().Options.DataDirectory
	return voice.SttRemove(dataDir, provider, model)
}

// SelectSttModel writes the engine choice and the settings that travel with
// it into options.voice, then publishes the config change the same way every
// other config writer does. The hosted engine carries no model. The language
// is always written (empty means auto-detect); the accelerator and GPU
// device only mean something to whisper, so they are written, cleared or
// left alone depending on what the request says.
func (b *Backend) SelectSttModel(workspaceID string, request proto.SttModelRequest) (proto.SttSelection, error) {
	provider, model := request.Provider, request.Model
	switch provider {
	case voice.EngineGoogle:
		model = ""
	case voice.EngineWhisper, voice.EngineVosk:
		if strings.TrimSpace(model) == "" {
			return proto.SttSelection{}, errors.New("model is required for local STT engines")
		}
	default:
		return proto.SttSelection{}, ErrSttUnknownProvider
	}
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.SttSelection{}, err
	}
	fields := map[string]any{
		"options.voice.engine":   provider,
		"options.voice.model":    model,
		"options.voice.language": strings.TrimSpace(request.Language),
	}
	if provider == voice.EngineWhisper {
		accelerator := sttConfiguredAccelerator(request.Accelerator)
		if accelerator != "" {
			fields["options.voice.accelerator"] = accelerator
		}
		if accelerator == voice.AcceleratorCPU {
			// A CPU build has no GPU to name, so a stale device index is
			// cleared instead of hiding in the config.
			fields["options.voice.gpu_device"] = ""
		} else if device, ok := sttGpuDevice(request.GPUDevice); ok {
			fields["options.voice.gpu_device"] = device
		}
	}
	if err := ws.Cfg.SetConfigFields(config.ScopeGlobal, fields); err != nil {
		return proto.SttSelection{}, err
	}
	publishConfigChanged(ws)
	return proto.SttSelection{Engine: provider, Model: model}, nil
}

// sttGpuDevice accepts a non-negative integer device index and returns it in
// canonical decimal form, so "007" in a request cannot come back out of the
// config looking like a different device. The same index space serves the
// CUDA and the Vulkan accelerator.
func sttGpuDevice(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	index, err := strconv.Atoi(trimmed)
	if err != nil || index < 0 {
		return "", false
	}
	return strconv.Itoa(index), true
}

// SttTasks lists the workspace's STT install tasks in the same shape the
// plugin task list uses.
func (b *Backend) SttTasks(workspaceID string) ([]proto.PluginTask, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	ws.pluginMu.Lock()
	defer ws.pluginMu.Unlock()
	result := []proto.PluginTask{}
	for name, task := range ws.pluginTasks {
		if !isSttTaskKey(name) {
			continue
		}
		result = append(result, sttTaskToProto(name, task.Snapshot()))
	}
	return result, nil
}

// sttTaskToProto renders a task snapshot in the shared task shape.
func sttTaskToProto(name string, snapshot plugins.Snapshot) proto.PluginTask {
	state := "running"
	if snapshot.State == plugins.TaskDone {
		state = "completed"
	}
	if snapshot.State == plugins.TaskFailed {
		state = "failed"
	}
	errorText := ""
	if snapshot.Err != nil {
		errorText = snapshot.Err.Error()
		state = "failed"
	}
	return proto.PluginTask{
		Name:  name,
		State: state,
		Phase: snapshot.Progress.Phase,
		Text:  snapshot.Text,
		Done:  snapshot.Progress.Done,
		Total: snapshot.Progress.Total,
		Error: errorText,
	}
}
