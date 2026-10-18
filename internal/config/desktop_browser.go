package config

import "os"

const (
	DesktopBrowserMCPName = "browser"
	desktopBrowserURLEnv  = "PRIME_DESKTOP_BROWSER_URL"
	desktopBrowserKeyEnv  = "PRIME_DESKTOP_BROWSER_TOKEN"
)

func (c *Config) addDesktopBrowserMCP() {
	url := os.Getenv(desktopBrowserURLEnv)
	token := os.Getenv(desktopBrowserKeyEnv)
	if url == "" || token == "" {
		return
	}
	if _, exists := c.MCP[DesktopBrowserMCPName]; exists {
		return
	}
	sessionless := true
	c.MCP[DesktopBrowserMCPName] = MCPConfig{
		Type:        MCPHttp,
		URL:         url,
		Headers:     map[string]string{"Authorization": "Bearer " + token},
		Sessionless: &sessionless,
		Timeout:     120,
	}
}
