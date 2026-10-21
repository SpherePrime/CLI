package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDesktopPromptIdentityAndCapabilities(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	t.Setenv("PRIME_DESKTOP_BROWSER_URL", "http://127.0.0.1/private-endpoint")
	t.Setenv("PRIME_DESKTOP_BROWSER_TOKEN", "private-desktop-token")
	env := testEnv(t)
	cfg := writeSmartToolsConfig(t, env.workingDir)
	for _, remote := range []bool{false, true} {
		if remote {
			cfg.Config().Options.RemoteWorkspace = &config.RemoteWorkspace{Host: "example.test", User: "dev", Root: "/project"}
		}
		for _, builder := range []func(...prompt.Option) (*prompt.Prompt, error){coderPrompt, planPrompt} {
			p, err := builder(prompt.WithWorkingDir(env.workingDir))
			require.NoError(t, err)
			body, err := p.Build(context.Background(), "mock", "mock-model", cfg)
			require.NoError(t, err)
			require.Contains(t, body, "Prime Desktop")
			require.NotContains(t, body, "running in the user's terminal")
			for _, capability := range []string{"<desktop_context>", "desktop_capabilities", "projects", "chats", "working context", "files", "terminal", "apps", "browser", "tabs", "windows", "GitHub", "restore", "automation", "SSH", "plugins", "voice", "model"} {
				require.Contains(t, body, capability, p.Name())
			}
			require.Contains(t, body, "UI features do not grant tool access")
			require.NotContains(t, body, "private-desktop-token")
			require.NotContains(t, body, "private-endpoint")
			if p.Name() == "plan" {
				require.Contains(t, body, "You are in read-only mode")
			}
		}
	}
}

func TestTerminalPromptDoesNotInferDesktopFromBrowserCredentials(t *testing.T) {
	for _, marker := range []string{"", "0", "true"} {
		t.Run("marker_"+marker, func(t *testing.T) {
			t.Setenv("PRIME_DESKTOP", marker)
			t.Setenv("PRIME_DESKTOP_BROWSER_URL", "http://127.0.0.1/browser")
			t.Setenv("PRIME_DESKTOP_BROWSER_TOKEN", "test-token")
			body := renderCoderPrompt(t, false)
			require.Contains(t, body, "running in the user's terminal")
			require.NotContains(t, body, "<desktop_context>")
		})
	}
}
