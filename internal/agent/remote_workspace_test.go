package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRemoteWorkspaceToolsExcludeLocalBypasses(t *testing.T) {
	coord := newGateTestCoordinator(t, true)
	require.NoError(t, coord.readyWg.Wait())
	coord.cfg.Config().Options.RemoteWorkspace = &config.RemoteWorkspace{Host: "host", User: "dev", Root: "/srv/project"}
	coord.cfg.Config().Options.SmartTools = true
	for _, agentName := range []string{config.AgentGeneral, config.AgentTask, config.AgentPlan, config.AgentCode} {
		for _, isSubAgent := range []bool{false, true} {
			built, err := coord.buildTools(context.Background(), coord.cfg.Config().Agents[agentName], isSubAgent)
			require.NoError(t, err)
			for _, tool := range built {
				require.Contains(t, []string{"todos", "fetch", "sourcegraph", "mcp_job", "question", "search_mcp", "search_tools"}, tool.Info().Name)
			}
			if search := builtToolByName(t, built, "search_tools"); search != nil {
				result, err := search.Run(context.Background(), fantasy.ToolCall{ID: "search", Input: `{"query":"shell bash edit agent lsp"}`})
				require.NoError(t, err)
				require.NotContains(t, result.Content, "Call bash")
				require.NotContains(t, result.Content, "Call agent")
				require.NotContains(t, result.Content, "Call edit")
			}
		}
	}
}

func TestRemoteWorkspacePromptUsesRemoteTarget(t *testing.T) {
	coord := newGateTestCoordinator(t, false)
	require.NoError(t, coord.readyWg.Wait())
	root := coord.cfg.WorkingDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("LOCAL_PROXY_ONLY_INSTRUCTIONS"), 0o600))
	coord.cfg.Config().Options.RemoteWorkspace = &config.RemoteWorkspace{Host: "remote.example", User: "dev", Root: "/srv/project"}
	coord.cfg.Config().Options.GlobalContextPaths = nil
	p, err := coderPrompt(prompt.WithWorkingDir(root))
	require.NoError(t, err)
	text, err := p.Build(context.Background(), "mock", "mock-model", coord.cfg)
	require.NoError(t, err)
	require.Contains(t, text, "Working directory: /srv/project")
	require.Contains(t, text, "Platform: linux")
	require.Contains(t, text, "LOCAL computer")
	require.Contains(t, text, "REMOTE Linux over SSH")
	require.Contains(t, text, "remote.example")
	require.Contains(t, text, "Never fall back")
	require.Contains(t, text, "remote AGENTS.md")
	require.NotContains(t, text, "LOCAL_PROXY_ONLY_INSTRUCTIONS")
	require.NotContains(t, text, root)
}
