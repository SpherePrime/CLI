package plugins

import (
	"path/filepath"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
)

// MigrateLegacy retires the short-lived "voice as an MCP server" experiment:
// a voice section from that era launched `prime mcp serve voice`, a command
// the plugin system replaced. Recognized legacy entries are removed from the
// mcp config (so the dead process never gets spawned again) and their
// installed state carries over to the plugin switch.
func MigrateLegacy(store *config.ConfigStore) {
	if store == nil {
		return
	}
	cfg := store.Config()
	if cfg == nil {
		return
	}
	entry, exists := cfg.MCP[VoiceName]
	if !exists || !isLegacyVoiceServer(entry) {
		return
	}
	if err := store.RemoveConfigField(config.ScopeGlobal, "mcp."+VoiceName); err != nil {
		return
	}
	if _, have := cfg.Plugins[VoiceName]; !have {
		_ = store.SetConfigField(config.ScopeGlobal, "plugins."+VoiceName,
			config.PluginConfig{Enabled: true})
	}
}

// isLegacyVoiceServer matches the exact config the old /mcp menu wrote: a
// stdio server whose command is a Prime binary and whose arguments are
// `mcp serve voice`. A user-authored server that merely shares the name is
// left alone.
func isLegacyVoiceServer(entry config.MCPConfig) bool {
	if entry.Type != config.MCPStdio {
		return false
	}
	if base := strings.ToLower(strings.TrimSuffix(filepath.Base(entry.Command), filepath.Ext(entry.Command))); base != "prime" && base != "cli" {
		return false
	}
	return len(entry.Args) == 3 &&
		entry.Args[0] == "mcp" &&
		entry.Args[1] == "serve" &&
		entry.Args[2] == VoiceName
}
