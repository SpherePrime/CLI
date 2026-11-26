package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func setBlenderEnvDetector(t *testing.T, fn func() BlenderEnvironment) {
	t.Helper()
	prev := blenderEnvDetector
	blenderEnvDetector = fn
	blenderEnvCache.mu.Lock()
	blenderEnvCache.known = false
	blenderEnvCache.mu.Unlock()
	t.Cleanup(func() {
		blenderEnvDetector = prev
		blenderEnvCache.mu.Lock()
		blenderEnvCache.known = false
		blenderEnvCache.mu.Unlock()
	})
}

func TestDesktopBlenderMCPIsAddedOutOfTheBoxWhenRunnerAvailable(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	setBlenderEnvDetector(t, func() BlenderEnvironment {
		return BlenderEnvironment{
			Installed:      false,
			MCPCommand:     "uvx",
			MCPArgs:        []string{"mcp-for-blender"},
			MCPAvailable:   true,
		}
	})

	cfg := &Config{MCP: map[string]MCPConfig{}}
	cfg.addDesktopBlenderMCP()

	entry, ok := cfg.MCP[BlenderMCPName]
	require.True(t, ok)
	require.Equal(t, MCPStdio, entry.Type)
	require.Equal(t, "uvx", entry.Command)
	require.Equal(t, []string{"mcp-for-blender"}, entry.Args)
	require.False(t, entry.Disabled)
	require.Equal(t, "1", entry.Env["PYTHONUTF8"])
}

func TestDesktopBlenderMCPNotAddedOutsideDesktopOrWithoutRunner(t *testing.T) {
	setBlenderEnvDetector(t, func() BlenderEnvironment {
		return BlenderEnvironment{
			Installed:    true,
			MCPCommand:   "uvx",
			MCPArgs:      []string{"mcp-for-blender"},
			MCPAvailable: true,
		}
	})

	t.Setenv(desktopBrowserURLEnv, "")
	t.Setenv(desktopBrowserKeyEnv, "")
	cfg := &Config{MCP: map[string]MCPConfig{}}
	cfg.addDesktopBlenderMCP()
	require.NotContains(t, cfg.MCP, BlenderMCPName)

	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	setBlenderEnvDetector(t, func() BlenderEnvironment {
		return BlenderEnvironment{Installed: true, MCPAvailable: false}
	})
	cfg2 := &Config{MCP: map[string]MCPConfig{}}
	cfg2.addDesktopBlenderMCP()
	require.NotContains(t, cfg2.MCP, BlenderMCPName)
}

func TestDesktopBlenderMCPPreservesCustomOrDisabledUserConfig(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	setBlenderEnvDetector(t, func() BlenderEnvironment {
		return BlenderEnvironment{
			Installed:    true,
			MCPCommand:   "uvx",
			MCPArgs:      []string{"mcp-for-blender"},
			MCPAvailable: true,
		}
	})

	cfg := &Config{MCP: map[string]MCPConfig{
		BlenderMCPName: {Type: MCPStdio, Command: "custom-blender", Disabled: true},
	}}
	cfg.addDesktopBlenderMCP()

	entry := cfg.MCP[BlenderMCPName]
	require.Equal(t, "custom-blender", entry.Command)
	require.True(t, entry.Disabled)
	require.False(t, cfg.IsBlenderMCPEnabled())
}
