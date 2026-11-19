package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/acp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestACPCmd_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	found, _, err := rootCmd.Find([]string{"acp"})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "acp", found.Use)
	require.Equal(t, "Start ACP (Agent Client Protocol) server", found.Short)

	yoloFlag := found.Flags().Lookup("yolo")
	require.NotNil(t, yoloFlag)
	require.Equal(t, "y", yoloFlag.Shorthand)

	inherited := found.InheritedFlags()
	require.NotNil(t, inherited.Lookup("cwd"))
	require.NotNil(t, inherited.Lookup("data-dir"))
	require.NotNil(t, inherited.Lookup("debug"))
}

func TestACPCmd_InitializeRoundtripNoStdoutContamination(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("LOCALAPPDATA", tmp)
	t.Setenv("PRIME_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")

	rootCmd.SetIn(stdin)
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"acp", "--cwd", tmp, "--data-dir", filepath.Join(tmp, ".prime")})
	t.Cleanup(func() {
		rootCmd.SetIn(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		_ = rootCmd.PersistentFlags().Set("cwd", "")
		_ = rootCmd.PersistentFlags().Set("data-dir", "")
		_ = rootCmd.PersistentFlags().Set("debug", "false")
		_ = acpCmd.Flags().Set("yolo", "false")
	})

	err := rootCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	rawOut := stdout.String()
	require.True(t, strings.HasSuffix(rawOut, "\n"))
	lines := strings.Split(strings.TrimSuffix(rawOut, "\n"), "\n")
	require.Len(t, lines, 1)

	var resp acp.RPCMessage
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &resp))
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.ID)
	require.Equal(t, "1", resp.ID.String())

	var initRes acp.InitializeResult
	require.NoError(t, json.Unmarshal(resp.Result, &initRes))
	require.Equal(t, acp.ProtocolVersion, initRes.ProtocolVersion)
	require.NotNil(t, initRes.AgentInfo)
	require.Equal(t, "prime", initRes.AgentInfo.Name)
	require.True(t, initRes.AgentCapabilities.LoadSession)
	require.Len(t, initRes.AuthMethods, 1)
	require.Equal(t, acp.AuthMethodPrimeGoogle, initRes.AuthMethods[0].ID)

	logPath := filepath.Join(config.GlobalCacheDir(), "acp", "prime.log")
	logBytes, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	require.Contains(t, string(logBytes), "Starting ACP server")
}
