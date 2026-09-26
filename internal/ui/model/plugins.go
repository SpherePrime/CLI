package model

import (
	"context"
	"fmt"

	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/ui/util"
)

// pluginInstallMsg reports that a background plugin install has finished.
type pluginInstallMsg struct {
	Name string
	Err  error
}

// togglePlugin flips a plugin's installed switch. Enabling kicks off the
// component download (if anything is missing) and reports back through
// pluginInstallMsg; disabling cancels an in-flight install and releases the
// plugin's background helpers. The config store is the live source the keymap
// reads, so the hotkey takes effect without a restart.
func (m *UI) togglePlugin(name string) tea.Cmd {
	plugin, ok := plugins.Get(name)
	if !ok {
		return util.ReportError(fmt.Errorf("unknown plugin %q", name))
	}

	if cfg := m.com.Config(); cfg.IsPluginEnabled(name) {
		if task := plugins.Running(name); task != nil {
			task.Cancel()
		}
		if err := m.setPluginEnabled(name, false); err != nil {
			return util.ReportError(err)
		}
		if plugin.Deactivate != nil {
			go plugin.Deactivate(context.Background())
		}
		return util.ReportInfo(m.com.LSprintf("info.plugin_disabled", m.pluginTitle(plugin)))
	}

	return tea.Batch(
		m.startPluginInstall(plugin),
		util.ReportInfo(m.com.LSprintf("info.plugin_enabling", m.pluginTitle(plugin))),
	)
}

// startPluginInstall launches (or joins) the install task and enables the
// plugin right away, so the plugins menu shows live progress while the
// download runs.
func (m *UI) startPluginInstall(plugin plugins.Plugin) tea.Cmd {
	settings := m.voiceSettings()
	task := plugins.InstallAsync(plugin, settings)
	if err := m.setPluginEnabled(plugin.Name, true); err != nil {
		task.Cancel()
		return util.ReportError(err)
	}
	return func() tea.Msg {
		<-task.Done()
		return pluginInstallMsg{Name: plugin.Name, Err: task.Snapshot().Err}
	}
}

// handlePluginInstall finishes the enable flow: a failed install switches the
// plugin back off, a successful one starts its background helpers and tells
// the user how to use it.
func (m *UI) handlePluginInstall(msg pluginInstallMsg) tea.Cmd {
	plugin, ok := plugins.Get(msg.Name)
	if !ok {
		return nil
	}
	title := m.pluginTitle(plugin)
	if msg.Err != nil {
		_ = m.setPluginEnabled(msg.Name, false)
		return util.ReportError(fmt.Errorf("%s: %w", title, msg.Err))
	}
	if plugin.Activate != nil {
		plugin.Activate(context.Background(), m.voiceSettings())
	}
	key := "info.plugin_ready"
	if msg.Name == plugins.VoiceName {
		key = "info.plugin_voice_ready"
	}
	return util.CmdHandler(util.NewInfoMsg(m.com.L(key)))
}

func (m *UI) setPluginEnabled(name string, enabled bool) error {
	return m.com.Workspace.SetConfigField(config.ScopeGlobal, "plugins."+name,
		config.PluginConfig{Enabled: enabled})
}

func (m *UI) pluginTitle(plugin plugins.Plugin) string {
	key := "plugins." + plugin.Name + ".title"
	if label := m.com.L(key); label != "" && label != key {
		return label
	}
	return plugin.Title
}

// voicePluginOn reports whether the voice plugin is installed. UI models
// without a workspace (tests, early boot) are treated as not installed.
func (m *UI) voicePluginOn() bool {
	if m.com == nil || m.com.Workspace == nil {
		return false
	}
	return m.com.Config().IsPluginEnabled(plugins.VoiceName)
}

// openPluginsDialog shows the plugins menu.
func (m *UI) openPluginsDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.PluginsID) {
		m.dialog.BringToFront(dialog.PluginsID)
		return nil
	}
	pluginsDialog, cmd := dialog.NewPlugins(m.com)
	m.dialog.OpenDialog(pluginsDialog)
	return cmd
}
