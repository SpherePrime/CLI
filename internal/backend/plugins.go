package backend

import (
	"context"
	"errors"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/proto"
)

var ErrPluginNotFound = errors.New("plugin not found")

func (b *Backend) ListPlugins(workspaceID string) ([]proto.PluginInfo, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	result := []proto.PluginInfo{}
	for _, plugin := range plugins.All() {
		result = append(result, proto.PluginInfo{Name: plugin.Name, Title: plugin.Title, Description: plugin.Description, Enabled: plugins.Enabled(ws.Cfg.Config(), plugin.Name)})
	}
	return result, nil
}

func (b *Backend) SetPluginEnabled(ctx context.Context, workspaceID, name string, enabled bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	plugin, ok := plugins.Get(name)
	if !ok {
		return ErrPluginNotFound
	}
	ws.pluginMu.Lock()
	defer ws.pluginMu.Unlock()
	if ws.pluginDesired == nil {
		ws.pluginDesired = make(map[string]bool)
	}
	ws.pluginDesired[name] = enabled
	if !enabled {
		if task := ws.pluginTasks[name]; task != nil {
			task.Cancel()
		}
		if err := ws.Cfg.SetConfigField(config.ScopeGlobal, "plugins."+name+".enabled", false); err != nil {
			return err
		}
		if plugin.Deactivate != nil {
			plugin.Deactivate(ctx)
		}
		publishConfigChanged(ws)
		return nil
	}
	if plugins.Enabled(ws.Cfg.Config(), name) {
		return nil
	}
	if task := ws.pluginTasks[name]; task != nil && task.Snapshot().State == plugins.TaskRunning {
		return nil
	}
	ws.pluginErrors.Delete(name)
	settings := plugins.SettingsFor(ws.Cfg.Config(), ws.Cfg.Resolver())
	task := plugins.InstallAsync(plugin, settings)
	if ws.pluginTasks == nil {
		ws.pluginTasks = make(map[string]*plugins.Task)
	}
	ws.pluginTasks[name] = task
	go func() {
		<-task.Done()
		ws.pluginMu.Lock()
		defer ws.pluginMu.Unlock()
		if task.Snapshot().Err != nil || ws.ctx.Err() != nil || !ws.pluginDesired[name] {
			return
		}
		if err := ws.Cfg.SetConfigField(config.ScopeGlobal, "plugins."+name+".enabled", true); err != nil {
			ws.pluginErrors.Store(name, err.Error())
			return
		}
		if plugin.Activate != nil {
			plugin.Activate(ws.ctx, settings)
		}
		publishConfigChanged(ws)
	}()
	return nil
}

func (b *Backend) PluginTasks(workspaceID string) ([]proto.PluginTask, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	ws.pluginMu.Lock()
	defer ws.pluginMu.Unlock()
	result := []proto.PluginTask{}
	for name, task := range ws.pluginTasks {
		// STT model installs travel in the same store under stt:* keys and
		// have their own endpoint; here only plugin installs are listed.
		if strings.HasPrefix(name, "stt:") {
			continue
		}
		snapshot := task.Snapshot()
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
		if value, ok := ws.pluginErrors.Load(name); ok {
			errorText = value.(string)
			state = "failed"
		}
		result = append(result, proto.PluginTask{Name: name, State: state, Phase: snapshot.Progress.Phase, Text: snapshot.Text, Done: snapshot.Progress.Done, Total: snapshot.Progress.Total, Error: errorText})
	}
	return result, nil
}
