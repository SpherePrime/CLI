package config

import "encoding/base64"

type RemoteWorkspace struct {
	Host string `json:"host"`
	User string `json:"user"`
	Root string `json:"root"`
}

func (o *Options) IsRemoteWorkspace() bool {
	return o != nil && o.RemoteWorkspace != nil
}

func (c *Config) IsDesktopMCP(name, workingDir string) bool {
	server, exists := c.MCP[name]
	if !exists {
		return false
	}
	url, token := desktopBrowserCredentials()
	return url != "" && token != "" && server.Type == MCPHttp && server.URL == url &&
		server.Headers["Authorization"] == "Bearer "+token &&
		server.Headers["X-Prime-Workspace"] == base64.RawURLEncoding.EncodeToString([]byte(workingDir))
}

func (c *Config) restrictRemoteWorkspace(workingDir string) {
	if !c.Options.IsRemoteWorkspace() {
		return
	}
	autoLSP := false
	c.Options.AutoLSP = &autoLSP
	c.LSP = map[string]LSPConfig{}
	c.Hooks = nil
	for name := range c.MCP {
		if !c.IsDesktopMCP(name, workingDir) {
			delete(c.MCP, name)
		}
	}
}
