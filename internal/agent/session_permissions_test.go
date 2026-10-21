package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSubAgentInheritsSessionAccessMode(t *testing.T) {
	for _, background := range []bool{false, true} {
		for _, fullAccess := range []bool{false, true} {
			name := "ask"
			if fullAccess {
				name = "full"
			}
			if background {
				name += "_background"
			}
			t.Run(name, func(t *testing.T) {
				env := testEnv(t)
				coord := newTestCoordinator(t, env, "test", config.ProviderConfig{ID: "test"})
				coord.permissions = permission.NewPermissionService(t.TempDir(), !fullAccess, nil)
				scoped := coord.permissions.(permission.SessionSkipService)
				parent, err := env.sessions.Create(t.Context(), "parent")
				require.NoError(t, err)
				scoped.SetSessionSkipRequests(parent.ID, fullAccess)
				agent := newMockAgent("test", 4096, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
					return agentResultWithText("ok"), nil
				})
				params := subAgentParams{
					Agent: agent, SessionID: parent.ID, AgentMessageID: "message", ToolCallID: "tool", SessionTitle: "child",
					SessionSetup: func(childID string) {
						require.Equal(t, fullAccess, scoped.SessionSkipRequests(childID))
						ctx, cancel := context.WithCancel(t.Context())
						cancel()
						granted, err := coord.permissions.Request(ctx, permission.CreatePermissionRequest{SessionID: childID, ToolName: "bash"})
						require.Equal(t, fullAccess, granted)
						if fullAccess {
							require.NoError(t, err)
						} else {
							require.ErrorIs(t, err, context.Canceled)
						}
					},
				}
				if background {
					job, err := coord.startAgentJob(t.Context(), params, "test")
					require.NoError(t, err)
					status, _ := waitForJob(t, coord, job.id)
					require.Equal(t, "done", status)
				} else {
					_, err := coord.runSubAgent(t.Context(), params)
					require.NoError(t, err)
				}
			})
		}
	}
}
