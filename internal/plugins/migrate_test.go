package plugins

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestIsLegacyVoiceServer(t *testing.T) {
	require.True(t, isLegacyVoiceServer(config.MCPConfig{
		Type:    config.MCPStdio,
		Command: `C:\Users\dev\prime.exe`,
		Args:    []string{"mcp", "serve", "voice"},
	}), "the exact section the old /mcp menu wrote is retired")

	require.True(t, isLegacyVoiceServer(config.MCPConfig{
		Type:    config.MCPStdio,
		Command: "/usr/local/bin/prime",
		Args:    []string{"mcp", "serve", "voice"},
	}))

	require.True(t, isLegacyVoiceServer(config.MCPConfig{
		Type:    config.MCPStdio,
		Command: "C:\\Tools\\Prime.EXE",
		Args:    []string{"mcp", "serve", "voice"},
	}), "matching must not depend on the OS that runs the migration")

	require.False(t, isLegacyVoiceServer(config.MCPConfig{
		Type:    config.MCPStdio,
		Command: "some-other-tool",
		Args:    []string{"mcp", "serve", "voice"},
	}), "a user-authored voice server with a different command stays untouched")

	require.False(t, isLegacyVoiceServer(config.MCPConfig{
		Type: config.MCPHttp,
		URL:  "http://localhost:9000/mcp",
	}), "http transports are never legacy entries")
}
