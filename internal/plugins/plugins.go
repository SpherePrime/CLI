// Package plugins is Prime's own optional-features system. A plugin lives
// inside the prime binary, is switched on from the plugins menu in the
// commands dialog, and downloads whatever it needs to work at the moment it
// is enabled. Nothing is installed, bound to keys, or downloaded by default.
//
// Plugins are unrelated to MCP servers, which stay a user-configurable
// integration; this package only ships first-party features.
package plugins

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/voice"
)

// Plugin describes one first-party feature and its lifecycle hooks.
type Plugin struct {
	// Name keys the plugins config section, the menu, and task tracking.
	Name string
	// Title is the English fallback label; the menu prefers the
	// "plugins.<name>.title" translation.
	Title string
	// Description explains the feature in the menu.
	Description string

	// Install brings in everything the plugin needs on this machine and
	// reports progress. It is idempotent: an already-complete machine
	// finishes immediately.
	Install func(ctx context.Context, settings voice.Settings, report func(Progress)) error
	// Activate starts long-running helpers (e.g. warming an engine) after
	// the plugin is switched on. Best-effort and must not block.
	Activate func(ctx context.Context, settings voice.Settings)
	// Deactivate stops background helpers when the plugin is switched off.
	Deactivate func(ctx context.Context)
}

// Progress is one structured update from an Install run.
type Progress struct {
	Phase string
	Text  string
	Done  int64
	Total int64
}

// All returns every plugin Prime ships, in menu order.
func All() []Plugin {
	return []Plugin{voicePlugin()}
}

// Get finds a plugin by name.
func Get(name string) (Plugin, bool) {
	for _, plugin := range All() {
		if plugin.Name == name {
			return plugin, true
		}
	}
	return Plugin{}, false
}

// Enabled reports whether a plugin is switched on in this config.
func Enabled(cfg *config.Config, name string) bool {
	return cfg.IsPluginEnabled(name)
}

// SettingsFor translates the engine options Prime shares with the voice
// plugin into pipeline settings.
func SettingsFor(cfg *config.Config, resolver voice.VariableResolver) voice.Settings {
	var options *config.VoiceOptions
	if cfg != nil && cfg.Options != nil {
		options = cfg.Options.Voice
	}
	return voice.SettingsFrom(options, resolver)
}
