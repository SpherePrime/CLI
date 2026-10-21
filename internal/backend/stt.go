package backend

import (
	"context"
	"errors"
	"log/slog"
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
// models and their install state.
func (b *Backend) SttCatalog(workspaceID string) (proto.SttCatalog, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.SttCatalog{}, err
	}
	cfg := ws.Cfg.Config()
	options := cfg.Options.Voice
	engine, model := "", ""
	if options != nil {
		// Lowercased like voice.SettingsFrom normalizes it, so the catalog
		// shows what dictation would actually run.
		engine = strings.ToLower(strings.TrimSpace(options.Engine))
		model = strings.TrimSpace(options.Model)
	}

	providers := voice.SttCatalog(cfg.Options.DataDirectory)
	result := proto.SttCatalog{Engine: engine, Model: model, Providers: make([]proto.SttProvider, 0, len(providers))}
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
		result.Providers = append(result.Providers, proto.SttProvider{
			ID:          provider.ID,
			Title:       provider.Title,
			Description: provider.Description,
			Available:   provider.Available,
			Models:      models,
		})
	}
	return result, nil
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
	dataDir := ws.Cfg.Config().Options.DataDirectory
	key := sttTaskKey(provider, model)

	ws.pluginMu.Lock()
	defer ws.pluginMu.Unlock()
	if task := ws.pluginTasks[key]; task != nil && task.Snapshot().State == plugins.TaskRunning {
		return ErrSttTaskRunning
	}
	plugin := plugins.Plugin{
		Name: key,
		Install: func(ctx context.Context, _ voice.Settings, report func(plugins.Progress)) error {
			return voice.SttInstall(ctx, dataDir, provider, model, func(progress voice.SetupProgress) {
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

// SelectSttModel writes the engine choice into options.voice and publishes
// the config change the same way every other config writer does. The
// hosted engine carries no model.
func (b *Backend) SelectSttModel(workspaceID, provider, model string) (proto.SttSelection, error) {
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
	if err := ws.Cfg.SetConfigFields(config.ScopeGlobal, map[string]any{
		"options.voice.engine": provider,
		"options.voice.model":  model,
	}); err != nil {
		return proto.SttSelection{}, err
	}
	publishConfigChanged(ws)
	return proto.SttSelection{Engine: provider, Model: model}, nil
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
