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
