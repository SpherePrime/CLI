package config

import (
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRemoteWorkspaceRestrictsLocalIntegrations(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "http://127.0.0.1:4000/mcp")
	t.Setenv(desktopBrowserKeyEnv, "desktop-token")
	root := t.TempDir()
	enabled := true
	cfg := &Config{
		Options: &Options{RemoteWorkspace: &RemoteWorkspace{Host: "host", User: "user", Root: "/srv/project"}, AutoLSP: &enabled},
		MCP:     map[string]MCPConfig{"custom": {Type: MCPStdio, Command: "local-command"}},
		LSP:     map[string]LSPConfig{"local": {Command: "local-lsp"}},
		Hooks:   map[string][]HookConfig{"PreToolUse": {{Command: "local-hook"}}},
	}
	cfg.setDefaults(root, "")
	require.False(t, *cfg.Options.AutoLSP)
	require.Empty(t, cfg.LSP)
	require.Empty(t, cfg.Hooks)
	require.Len(t, cfg.MCP, 1)
	require.True(t, cfg.IsDesktopMCP(DesktopBrowserMCPName, root))
	require.False(t, cfg.IsDesktopMCP(DesktopBrowserMCPName, t.TempDir()))
	server := cfg.MCP[DesktopBrowserMCPName]
	server.Headers["Authorization"] = "Bearer forged"
	cfg.MCP[DesktopBrowserMCPName] = server
	require.False(t, cfg.IsDesktopMCP(DesktopBrowserMCPName, root))
}

func TestRemoteWorkspaceWithoutDesktopFailsClosed(t *testing.T) {
	t.Setenv(desktopBrowserURLEnv, "")
	t.Setenv(desktopBrowserKeyEnv, "")
	cfg := &Config{
		Options: &Options{RemoteWorkspace: &RemoteWorkspace{}},
		MCP:     map[string]MCPConfig{DesktopBrowserMCPName: {Type: MCPHttp, URL: "http://untrusted/mcp"}},
	}
	cfg.setDefaults(t.TempDir(), "")
	require.Empty(t, cfg.MCP)
	require.False(t, *cfg.Options.AutoLSP)
	require.True(t, cfg.Options.IsRemoteWorkspace())
}

func TestRemoteWorkspaceConfigContract(t *testing.T) {
	var options Options
	require.NoError(t, json.Unmarshal([]byte(`{"remote_workspace":{"host":"example.org","user":"dev","root":"/srv/app"},"auto_lsp":false}`), &options))
	require.Equal(t, &RemoteWorkspace{Host: "example.org", User: "dev", Root: "/srv/app"}, options.RemoteWorkspace)
	encoded, err := json.Marshal(options.RemoteWorkspace)
	require.NoError(t, err)
	require.JSONEq(t, `{"host":"example.org","user":"dev","root":"/srv/app"}`, string(encoded))
}
