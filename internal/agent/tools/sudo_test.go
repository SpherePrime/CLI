package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func runSudoTool(t *testing.T, tool fantasy.AgentTool, params SudoParams) fantasy.ToolResponse {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	input, err := json.Marshal(params)
	require.NoError(t, err)
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: SudoToolName, Input: string(input)})
	require.NoError(t, err)
	return response
}

func TestSudoTool_NotConfiguredReturnsSettingsLink(t *testing.T) {
	t.Setenv(sudoPasswordEnv, "")
	workingDir := t.TempDir()
	tool := NewSudoTool(&mockBashPermissionService{}, workingDir, t.TempDir())

	response := runSudoTool(t, tool, SudoParams{Description: "check", Command: "whoami"})

	require.True(t, response.IsError)
	require.Contains(t, response.Content, SudoSettingsLink)
}

func TestSudoTool_MissingCommandFails(t *testing.T) {
	t.Setenv(sudoPasswordEnv, "secret")
	tool := NewSudoTool(&mockBashPermissionService{}, t.TempDir(), t.TempDir())

	response := runSudoTool(t, tool, SudoParams{Description: "empty", Command: "  "})

	require.True(t, response.IsError)
	require.Contains(t, response.Content, "missing command")
}

func TestSudoTool_WrapsCommandWithAskpass(t *testing.T) {
	askpassPath := filepath.Join(t.TempDir(), "askpass")
	wrapped := sudoCommand("apt-get install -y ripgrep", askpassPath)

	require.Contains(t, wrapped, "sudo -A -p '' -- bash -c")
	require.Contains(t, wrapped, "'apt-get install -y ripgrep'")
	require.NotContains(t, wrapped, "echo")
}

func TestSudoCommand_QuotesSingleQuotes(t *testing.T) {
	wrapped := sudoCommand("echo 'hi there'", "/tmp/askpass")

	require.Contains(t, wrapped, `'echo '\''hi there'\'''`)
}

func TestWriteAskpassScript_EmitsPasswordAndIsRemoved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("askpass scripts need POSIX executable bits")
	}
	password := "it's a secret"
	path, err := writeAskpassScript(password)
	require.NoError(t, err)
	require.NotNil(t, os.Remove)
	defer func() { require.NoError(t, os.Remove(path)) }()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), `echo 'it'\''s a secret'`)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0o077, "askpass script must be owner-only")
}

func TestSudoTool_DescribesUsage(t *testing.T) {
	description := NewSudoTool(&mockBashPermissionService{}, t.TempDir(), t.TempDir()).Info().Description

	require.True(t, strings.Contains(description, "root privileges"))
	require.Contains(t, description, "WITHOUT the sudo prefix")
}
