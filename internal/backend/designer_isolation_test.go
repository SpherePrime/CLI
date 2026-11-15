package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func createHermeticWorkspace(t *testing.T, b *Backend, kind string) (*Workspace, proto.Workspace) {
	t.Helper()
	project := t.TempDir()
	dataDir := t.TempDir()
	configuration := `{
		"options":{"disable_default_providers":true,"disable_provider_auto_update":true,"disable_metrics":true},
		"providers":{"fixture":{"id":"fixture","type":"openai-compat","base_url":"http://127.0.0.1:9/v1","api_key":"fixture-key","discover_models":false,"models":[{"id":"fixture-model","name":"Fixture","context_window":128000,"default_max_tokens":8192},{"id":"fixture-small","name":"Small","context_window":128000,"default_max_tokens":4096}]}},
		"models":{"large":{"provider":"fixture","model":"fixture-model"},"small":{"provider":"fixture","model":"fixture-small"}}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(project, "prime.json"), []byte(configuration), 0o600))

	args := protoWS(project, dataDir, uuid.New().String())
	args.Kind = kind
	ws, p, err := b.CreateWorkspace(args)
	require.NoError(t, err)
	return ws, p
}

func TestWorkspaceKindIsolation_NonDesignerWorkspaces(t *testing.T) {
	for _, kind := range []string{"", "project", "chat"} {
		name := kind
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			xdgIsolated(t)
			t.Setenv("USERPROFILE", t.TempDir())
			t.Setenv("APPDATA", t.TempDir())
			t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
			t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

			b := New(context.Background(), nil, func() {})
			b.SetCreateGrace(2 * time.Second)
			t.Cleanup(func() { drainBackend(t, b) })

			ws, p := createHermeticWorkspace(t, b, kind)
			require.Equal(t, kind, ws.Kind)
			require.Equal(t, kind, p.Kind)
			require.Empty(t, ws.mainAgent)

			err := b.SetMainAgent(ws.ID, config.AgentDesigner)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
			require.Empty(t, ws.mainAgent)

			require.NoError(t, b.InitAgent(context.Background(), ws.ID, true))
			require.NotNil(t, ws.Coordinator())

			err = b.SetMainAgent(ws.ID, config.AgentDesigner)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
			require.Empty(t, ws.mainAgent)

			require.NoError(t, b.SetMainAgent(ws.ID, config.AgentPlan))
			require.Equal(t, config.AgentPlan, ws.mainAgent)
			require.NoError(t, b.SetMainAgent(ws.ID, config.AgentGeneral))
			require.Equal(t, config.AgentGeneral, ws.mainAgent)
		})
	}
}

func TestWorkspaceKindIsolation_DesignerWorkspace(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	ws, p := createHermeticWorkspace(t, b, "designer")
	require.Equal(t, "designer", ws.Kind)
	require.Equal(t, "designer", p.Kind)
	require.Equal(t, config.AgentDesigner, ws.mainAgent)

	for _, disallowed := range []string{config.AgentGeneral, config.AgentPlan, config.AgentTask} {
		err := b.SetMainAgent(ws.ID, disallowed)
		require.Error(t, err, fmt.Sprintf("expected error switching designer workspace to %s", disallowed))
		require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
		require.Equal(t, config.AgentDesigner, ws.mainAgent)
	}

	require.NoError(t, b.SetMainAgent(ws.ID, config.AgentDesigner))
	require.Equal(t, config.AgentDesigner, ws.mainAgent)

	require.NoError(t, b.InitAgent(context.Background(), ws.ID, true))
	require.NotNil(t, ws.Coordinator())

	for _, disallowed := range []string{config.AgentGeneral, config.AgentPlan, config.AgentTask} {
		err := b.SetMainAgent(ws.ID, disallowed)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
		require.Equal(t, config.AgentDesigner, ws.mainAgent)
	}
}

func TestWorkspaceKindIsolation_CoordinatorAndSendMessageDefenseInDepth(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)

	projectCoord := newBlockingCoordinator()
	projectWS := insertAgentWorkspace(t, b, projectCoord)
	projectWS.Kind = "project"

	err := b.SetMainAgent(projectWS.ID, config.AgentDesigner)
	require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
	require.Nil(t, projectCoord.lastMainAgentSet.Load())

	projectWS.mainAgent = config.AgentDesigner
	err = b.SendMessage(projectWS.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"})
	require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)

	designerCoord := newBlockingCoordinator()
	designerWS := insertAgentWorkspace(t, b, designerCoord)
	designerWS.Kind = "designer"
	designerWS.mainAgent = config.AgentDesigner

	err = b.SetMainAgent(designerWS.ID, config.AgentGeneral)
	require.ErrorIs(t, err, ErrAgentNotAllowedForWorkspace)
	require.Nil(t, designerCoord.lastMainAgentSet.Load())
}

func TestWorkspaceKindIsolation_DuplicatePathKindConflict(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	dirA := t.TempDir()
	dataA := t.TempDir()
	argsProject := protoWS(dirA, dataA, uuid.New().String())
	argsProject.Kind = "project"
	wsProject, _, err := b.CreateWorkspace(argsProject)
	require.NoError(t, err)

	argsDesigner := protoWS(dirA, dataA, uuid.New().String())
	argsDesigner.Kind = "designer"
	_, _, err = b.CreateWorkspace(argsDesigner)
	require.ErrorIs(t, err, ErrWorkspaceKindMismatch)

	argsCLI := protoWS(dirA, dataA, uuid.New().String())
	wsDedupe, _, err := b.CreateWorkspace(argsCLI)
	require.NoError(t, err)
	require.Equal(t, wsProject.ID, wsDedupe.ID)

	dirB := t.TempDir()
	dataB := t.TempDir()
	bDesigner := protoWS(dirB, dataB, uuid.New().String())
	bDesigner.Kind = "designer"
	_, _, err = b.CreateWorkspace(bDesigner)
	require.NoError(t, err)

	bProject := protoWS(dirB, dataB, uuid.New().String())
	bProject.Kind = "project"
	_, _, err = b.CreateWorkspace(bProject)
	require.ErrorIs(t, err, ErrWorkspaceKindMismatch)
}
