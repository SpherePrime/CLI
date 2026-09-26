package mcps

import (
	"os"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"

	"github.com/SpherePrime/CLI/internal/config"
)

func TestVoiceConfigLaunchesPrimeItself(t *testing.T) {
	entry, err := voiceConfig()
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)

	require.Equal(t, config.MCPStdio, entry.Type)
	require.Equal(t, exe, entry.Command)
	require.Equal(t, []string{"mcp", "serve", ServerNameVoice}, entry.Args)
	require.Greater(t, entry.Timeout, 10, "dictations outlive the default mcp timeout")
	require.False(t, entry.Disabled)
}

func TestLookupFindsVoice(t *testing.T) {
	server, ok := Lookup(ServerNameVoice)
	require.True(t, ok)
	require.NotNil(t, server.BuildConfig)
	require.NotNil(t, server.Serve)
	require.NotEmpty(t, server.Tools)

	_, ok = Lookup("nope")
	require.False(t, ok)
}
