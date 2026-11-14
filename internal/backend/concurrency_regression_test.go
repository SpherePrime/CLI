package backend

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWorkspaceCoordinatorConcurrentInitAndReads(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	project := t.TempDir()
	configuration := fmt.Sprintf(`{
		"options":{"disable_default_providers":true,"disable_provider_auto_update":true,"disable_metrics":true},
		"providers":{"fixture":{"id":"fixture","type":"openai-compat","base_url":%q,"api_key":"fixture-key","discover_models":false,"models":[{"id":"fixture-model","name":"Fixture","context_window":128000,"default_max_tokens":8192},{"id":"fixture-small","name":"Small","context_window":128000,"default_max_tokens":4096}]}},
		"models":{"large":{"provider":"fixture","model":"fixture-model"},"small":{"provider":"fixture","model":"fixture-small"}}
	}`, server.URL)
	require.NoError(t, os.WriteFile(filepath.Join(project, "prime.json"), []byte(configuration), 0o600))

	b := New(context.Background(), nil, func() {})
	t.Cleanup(func() { drainBackend(t, b) })

	ws, _, err := b.CreateWorkspace(protoWS(project, t.TempDir(), uuid.New().String()))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for range 25 {
				_, _ = b.GetAgentInfo(ws.ID)
				_, _ = b.QueuedPrompts(ws.ID, "S1")
				_, _ = b.QueuedPromptsList(ws.ID, "S1")
				_ = b.CancelSession(ws.ID, "S1")
				_, _ = b.QueueEntries(ws.ID, "S1")
				_, _ = b.GetAgentWorkers(context.Background(), ws.ID, "S1")
				_ = ws.Coordinator()
				if i%2 == 0 {
					_ = b.InitAgent(context.Background(), ws.ID, true)
				}
			}
		})
	}
	wg.Wait()
}

func TestWorkspaceSetCoordinatorConcurrentWithReaders(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	coord := newBlockingCoordinator()
	ws := insertAgentWorkspace(t, b, nil)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				if i == 0 {
					if j%2 == 0 {
						ws.App.SetCoordinator(coord)
					} else {
						ws.App.SetCoordinator(nil)
					}
				}
				_, _ = b.GetAgentInfo(ws.ID)
				_, _ = b.QueuedPrompts(ws.ID, "S1")
				_, _ = b.QueuedPromptsList(ws.ID, "S1")
				_ = b.CancelSession(ws.ID, "S1")
				_ = b.ClearQueue(ws.ID, "S1")
				_, _ = b.QueueEntries(ws.ID, "S1")
			}
		})
	}
	wg.Wait()
}

func TestProjectMetaActiveQuerySurvivesConcurrentEvictionAndCloseAll(t *testing.T) {
	t.Cleanup(CloseAllProjectMetaConns)
	CloseAllProjectMetaConns()

	project := t.TempDir()
	dataDir := filepath.Join(project, ".prime")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	rwConn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	_, err = db.New(rwConn).CreateSession(context.Background(), db.CreateSessionParams{
		ID:    "active-query-session",
		Title: "Active query",
	})
	require.NoError(t, err)
	require.NoError(t, db.Release(dataDir))

	absPath, err := filepath.Abs(project)
	require.NoError(t, err)
	dbPath := filepath.Join(absPath, ".prime", "prime.db")

	conn, release, err := acquireProjectMetaConn(absPath, dbPath)
	require.NoError(t, err)

	rows, err := conn.QueryContext(context.Background(), projectSessionsQuery)
	require.NoError(t, err)

	CloseAllProjectMetaConns()

	require.True(t, rows.Next())
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	release()

	require.Error(t, conn.PingContext(context.Background()))
}

func TestProjectMetaConcurrentQueriesAcrossMoreThanMaxConns(t *testing.T) {
	t.Cleanup(CloseAllProjectMetaConns)
	CloseAllProjectMetaConns()

	projects := make([]string, projectMetaMaxConns+4)
	for i := range projects {
		project := t.TempDir()
		dataDir := filepath.Join(project, ".prime")
		require.NoError(t, os.MkdirAll(dataDir, 0o755))
		conn, err := db.Connect(context.Background(), dataDir)
		require.NoError(t, err)
		_, err = db.New(conn).CreateSession(context.Background(), db.CreateSessionParams{
			ID:    fmt.Sprintf("s-%d", i),
			Title: fmt.Sprintf("Project %d", i),
		})
		require.NoError(t, err)
		require.NoError(t, db.Release(dataDir))
		projects[i] = project
	}

	b, _ := newTestBackend(t)
	var wg sync.WaitGroup
	for g := range 6 {
		wg.Go(func() {
			for round := range 10 {
				project := projects[(g*7+round)%len(projects)]
				sessions, err := b.ProjectSessions(context.Background(), project)
				require.NoError(t, err)
				require.Len(t, sessions, 1)
				if round%5 == 0 {
					CloseIdleProjectMetaConns()
				}
			}
		})
	}
	wg.Wait()
	CloseAllProjectMetaConns()
}

func TestCreateWorkspaceEvictsCachedProjectMetaAndShutdownClosesAll(t *testing.T) {
	xdgIsolated(t)
	t.Cleanup(CloseAllProjectMetaConns)
	CloseAllProjectMetaConns()

	project := t.TempDir()
	dataDir := filepath.Join(project, ".prime")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	_, err = db.New(conn).CreateSession(context.Background(), db.CreateSessionParams{
		ID:    "cached-session",
		Title: "Cached",
	})
	require.NoError(t, err)
	require.NoError(t, db.Release(dataDir))

	b := New(context.Background(), nil, func() {})
	t.Cleanup(b.CloseInProcess)

	sessions, err := b.ProjectSessions(context.Background(), project)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	absProject, err := filepath.Abs(project)
	require.NoError(t, err)
	projectMeta.mu.Lock()
	_, cachedBefore := projectMeta.entries[absProject]
	projectMeta.mu.Unlock()
	require.True(t, cachedBefore)

	ws, _, err := b.CreateWorkspace(proto.Workspace{
		Path:     project,
		DataDir:  dataDir,
		ClientID: uuid.New().String(),
	})
	require.NoError(t, err)
	require.NotNil(t, ws)

	projectMeta.mu.Lock()
	_, cachedAfterCreate := projectMeta.entries[absProject]
	projectMeta.mu.Unlock()
	require.False(t, cachedAfterCreate)

	otherProject := t.TempDir()
	otherDataDir := filepath.Join(otherProject, ".prime")
	require.NoError(t, os.MkdirAll(otherDataDir, 0o755))
	otherConn, err := db.Connect(context.Background(), otherDataDir)
	require.NoError(t, err)
	_, err = db.New(otherConn).CreateSession(context.Background(), db.CreateSessionParams{
		ID:    "other-session",
		Title: "Other",
	})
	require.NoError(t, err)
	require.NoError(t, db.Release(otherDataDir))

	_, err = b.ProjectSessions(context.Background(), otherProject)
	require.NoError(t, err)

	b.SetCreateGrace(50 * time.Millisecond)
	b.CloseInProcess()

	projectMeta.mu.Lock()
	remaining := len(projectMeta.entries)
	projectMeta.mu.Unlock()
	require.Zero(t, remaining)
	_ = config.AgentGeneral
}
