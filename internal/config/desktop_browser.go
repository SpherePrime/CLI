package config

import (
	"encoding/base64"
	"os"
	"strconv"
)

const (
	DesktopBrowserMCPName = "browser"
	desktopBrowserURLEnv  = "PRIME_DESKTOP_BROWSER_URL"
	desktopBrowserKeyEnv  = "PRIME_DESKTOP_BROWSER_TOKEN"
)

func desktopBrowserCredentials() (string, string) {
	return os.Getenv(desktopBrowserURLEnv), os.Getenv(desktopBrowserKeyEnv)
}

func (c *Config) addDesktopBrowserMCP(workingDir string) {
	url, token := desktopBrowserCredentials()
	if url == "" || token == "" {
		return
	}
	name := DesktopBrowserMCPName
	for suffix := 1; ; suffix++ {
		existing, exists := c.MCP[name]
		if !exists || existing.URL == url && existing.Headers["Authorization"] == "Bearer "+token {
			break
		}
		name = "prime-desktop"
		if suffix > 1 {
			name += "-" + strconv.Itoa(suffix)
		}
	}
	sessionless := true
	c.MCP[name] = MCPConfig{
		Type: MCPHttp,
		URL:  url,
		Headers: map[string]string{
			"Authorization":     "Bearer " + token,
			"X-Prime-Workspace": base64.RawURLEncoding.EncodeToString([]byte(workingDir)),
		},
		Sessionless: &sessionless,
		Timeout:     120,
	}
}
