package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDesktopBrowserMCPIsAddedOnlyWhenDesktopProvidesIt(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "")
	t.Setenv(desktopBrowserKeyEnv, "")
	cfg := &Config{MCP: map[string]MCPConfig{}}
	cfg.addDesktopBrowserMCP()
	require.NotContains(t, cfg.MCP, DesktopBrowserMCPName)

	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "secret")
	cfg.addDesktopBrowserMCP()
	server := cfg.MCP[DesktopBrowserMCPName]
	require.Equal(t, MCPHttp, server.Type)
	require.Equal(t, "Bearer secret", server.Headers["Authorization"])
	require.True(t, *server.Sessionless)
}
