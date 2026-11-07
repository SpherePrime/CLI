package config

import (
	"encoding/base64"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDesktopBrowserBindsToolsToWorkspaceRoot(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	root := t.TempDir()
	cfg := &Config{MCP: map[string]MCPConfig{}}
	cfg.addDesktopBrowserMCP(root)
	encoded := cfg.MCP[DesktopBrowserMCPName].Headers["X-Prime-Workspace"]
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.Equal(t, root, string(decoded))
}

func TestDesktopToolsPreserveCustomBrowserServer(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	cfg := &Config{MCP: map[string]MCPConfig{
		DesktopBrowserMCPName: {Type: MCPHttp, URL: "https://browser.example/mcp"},
	}}
	cfg.addDesktopBrowserMCP(t.TempDir())
	require.Equal(t, "https://browser.example/mcp", cfg.MCP[DesktopBrowserMCPName].URL)
	require.Equal(t, "http://127.0.0.1:4000/mcp", cfg.MCP["prime-desktop"].URL)
	root := t.TempDir()
	cfg.addDesktopBrowserMCP(root)
	require.Len(t, cfg.MCP, 2)
	require.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(root)), cfg.MCP["prime-desktop"].Headers["X-Prime-Workspace"])
}

func TestDesktopBrowserMCPIsAddedOnlyWhenDesktopProvidesIt(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "")
	t.Setenv(desktopBrowserKeyEnv, "")
	cfg := &Config{MCP: map[string]MCPConfig{}}
	cfg.addDesktopBrowserMCP(t.TempDir())
	require.NotContains(t, cfg.MCP, DesktopBrowserMCPName)

	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	cfg.addDesktopBrowserMCP(t.TempDir())
	server := cfg.MCP[DesktopBrowserMCPName]
	require.Equal(t, MCPHttp, server.Type)
	require.Equal(t, "Bearer secret", server.Headers["Authorization"])
	require.True(t, *server.Sessionless)
}

func TestDesktopMCPSplitsIntoGroupedAreas(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	t.Setenv(desktopMCPAreasEnv, "browser, apps,terminal,bad/area")
	root := t.TempDir()
	cfg := &Config{MCP: map[string]MCPConfig{"terminal": {Type: MCPStdio, Command: "custom"}}}
	cfg.addDesktopBrowserMCP(root)
	require.Equal(t, "http://127.0.0.1:4000/mcp/browser", cfg.MCP["browser"].URL)
	require.Equal(t, "http://127.0.0.1:4000/mcp/apps", cfg.MCP["apps"].URL)
	require.Equal(t, "custom", cfg.MCP["terminal"].Command)
	require.Equal(t, "http://127.0.0.1:4000/mcp/terminal", cfg.MCP["prime-desktop-terminal"].URL)
	require.Len(t, cfg.MCP, 4)
	for _, name := range []string{"browser", "apps", "prime-desktop-terminal"} {
		require.Equal(t, "grouped", cfg.MCP[name].Context)
		require.True(t, cfg.IsDesktopMCP(name, root))
	}
	require.False(t, cfg.IsDesktopMCP("terminal", root))
}
