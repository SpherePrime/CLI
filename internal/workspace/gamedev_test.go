package workspace

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestClientWorkspaceRespectsGameDevOverrideOverSessionConfig(t *testing.T) {
	t.Parallel()

	gdTrue := true
	cw := NewClientWorkspace(nil, proto.Workspace{
		ID:      "ws-1",
		Path:    t.TempDir(),
		GameDev: &gdTrue,
		Config: &config.Config{
			Options: &config.Options{GameDev: false},
			SessionGameDev: map[string]bool{
				"chat-off": false,
			},
		},
	})

	require.True(t, cw.Config().IsGameDevEnabled(""))
	require.True(t, cw.Config().IsGameDevEnabled("chat-off"))

	cw.Config().SetGameDevOverride(nil)
	require.False(t, cw.Config().IsGameDevEnabled(""))
	require.False(t, cw.Config().IsGameDevEnabled("chat-off"))
}
