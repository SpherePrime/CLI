package config

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"
)

const (
	DesktopBrowserMCPName = "browser"
	desktopBrowserURLEnv  = "PRIME_DESKTOP_BROWSER_URL"
	desktopBrowserKeyEnv  = "PRIME_DESKTOP_BROWSER_TOKEN"
	desktopMCPAreasEnv    = "PRIME_DESKTOP_MCP_AREAS"
	desktopMCPPrefix      = "prime-desktop"
	mcpContextGroupedMode = "grouped"
)

func desktopBrowserCredentials() (string, string) {
	return os.Getenv(desktopBrowserURLEnv), os.Getenv(desktopBrowserKeyEnv)
}

func desktopMCPAreas() []string {
	var areas []string
	for _, area := range strings.Split(os.Getenv(desktopMCPAreasEnv), ",") {
		area = strings.TrimSpace(area)
		if area != "" && !strings.ContainsAny(area, "/?#") {
			areas = append(areas, area)
		}
	}
	return areas
}

func (c *Config) addDesktopBrowserMCP(workingDir string) {
	url, token := desktopBrowserCredentials()
	if url == "" || token == "" {
		return
	}
	areas := desktopMCPAreas()
	if len(areas) == 0 {
		c.addDesktopMCPServer(DesktopBrowserMCPName, url, token, workingDir, "")
		return
	}
	for _, area := range areas {
		c.addDesktopMCPServer(area, url+"/"+area, token, workingDir, mcpContextGroupedMode)
	}
}

func (c *Config) addDesktopMCPServer(preferred, url, token, workingDir, context string) {
	name := preferred
	for suffix := 1; ; suffix++ {
		existing, exists := c.MCP[name]
		if !exists || existing.URL == url && existing.Headers["Authorization"] == "Bearer "+token {
			break
		}
		name = desktopMCPPrefix
		if preferred != DesktopBrowserMCPName {
			name += "-" + preferred
		}
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
		Context:     context,
	}
}

func isDesktopMCPURL(serverURL, desktopURL string) bool {
	return serverURL == desktopURL || strings.HasPrefix(serverURL, desktopURL+"/")
}
