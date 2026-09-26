// Package mcps is the home for Prime's own MCP servers. Each entry here is a
// stdio server Prime ships in its own binary and can offer from the /mcp
// menu: installing one only writes an mcp config section pointing back at
// `prime mcp serve <name>`, so there is nothing extra to download or keep
// updated.
package mcps

import (
	"context"
	"fmt"
	"os"

	"github.com/SpherePrime/CLI/internal/config"
	voice "github.com/SpherePrime/CLI/internal/mcps/voice"
)

// ServerNameVoice is both the registry key and the mcp section name Prime's
// dictation server installs under.
const ServerNameVoice = "voice"

// voiceToolTimeoutSeconds bounds one MCP round trip. A dictation records for
// up to a few minutes and then transcribes, so this is well above the
// default 10 seconds.
const voiceToolTimeoutSeconds = 300

// Server describes one first-party MCP server Prime can offer.
type Server struct {
	// Name keys the registry, the `prime mcp serve` argument, and the mcp
	// config section written on install.
	Name string
	// Title is the human label shown in the /mcp menu.
	Title string
	// Description explains what installing the server gives the agent.
	Description string
	// Tools lists the tool names the server exposes, for display.
	Tools []string
	// BuildConfig returns the mcp config section that launches this server.
	BuildConfig func() (config.MCPConfig, error)
	// Serve runs the server over stdio until ctx is done.
	Serve func(ctx context.Context) error
}

// Servers is everything Prime ships, in menu order.
func Servers() []Server {
	return []Server{
		{
			Name:        ServerNameVoice,
			Title:       "Voice dictation (Whisper)",
			Description: "Microphone dictation for agents: record until the speaker goes quiet, transcribe with a resident Whisper server, plus setup, warm-up, and status tools.",
			Tools:       []string{"dictate", "setup", "status", "warm", "stop_server"},
			BuildConfig: voiceConfig,
			Serve:       voice.Serve,
		},
	}
}

// Lookup finds a server by name.
func Lookup(name string) (Server, bool) {
	for _, server := range Servers() {
		if server.Name == name {
			return server, true
		}
	}
	return Server{}, false
}

// voiceConfig points Prime's own binary back at the voice server. The path is
// resolved at install time so the entry survives as a plain stdio server.
func voiceConfig() (config.MCPConfig, error) {
	exe, err := os.Executable()
	if err != nil {
		return config.MCPConfig{}, fmt.Errorf("cannot locate the prime executable: %w", err)
	}
	return config.MCPConfig{
		Type:    config.MCPStdio,
		Command: exe,
		Args:    []string{"mcp", "serve", ServerNameVoice},
		Timeout: voiceToolTimeoutSeconds,
	}, nil
}
