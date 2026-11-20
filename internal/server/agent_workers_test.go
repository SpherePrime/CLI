package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type workerCoordinatorStub struct {
	agent.Coordinator
	workers []agent.AgentWorker
}

func (s *workerCoordinatorStub) AgentWorkers(context.Context, string) ([]agent.AgentWorker, error) {
	return s.workers, nil
}

func (s *workerCoordinatorStub) SendAgentWorkerMessage(_ context.Context, _, _, _ string) (agent.AgentWorker, error) {
	return s.workers[0], nil
}

func (s *workerCoordinatorStub) StopAgentWorker(_ context.Context, _, _ string) error {
	return nil
}

func TestAgentWorkerErrorStatuses(t *testing.T) {
	c := &controllerV1{}
	for _, tc := range []struct {
		err    error
		status int
	}{{backend.ErrAgentWorkerNotFound, 404}, {backend.ErrAgentWorkersUnsupported, 409}, {backend.ErrAgentWorkerInput, 400}} {
		w := httptest.NewRecorder()
		c.workerError(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
		require.Equal(t, tc.status, w.Code)
	}
}

func TestAgentWorkerRoutesAndMalformedBody(t *testing.T) {
	c := &controllerV1{}
	mux := http.NewServeMux()
	for _, endpoint := range c.agentWorkerEndpoints() {
		mux.HandleFunc(endpoint.Method()+" "+endpoint.Path(), endpoint.Handler())
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/workspaces/workspace/agent/sessions/root/workers/child/messages", strings.NewReader("{")))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentWorkerMessagesPaginationAndLegacyFallback(t *testing.T) {
	c, workspaceID := buildBusyWorkspace(t, "unused", false)
	ws, err := c.backend.GetWorkspace(workspaceID)
	require.NoError(t, err)
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	queries := db.New(conn)
	ws.Sessions = session.NewService(queries, conn)
	ws.Messages = message.NewService(queries)
	rootSess, err := ws.Sessions.Create(t.Context(), "root")
	require.NoError(t, err)
	workerSess, err := ws.Sessions.Create(t.Context(), "worker")
	require.NoError(t, err)
	ws.App.SetCoordinator(&workerCoordinatorStub{
		workers: []agent.AgentWorker{{ID: "worker-1", SessionID: workerSess.ID, RootSessionID: rootSess.ID}},
	})
	ids := make([]string, 9)
	for i := range ids {
		msg, err := ws.Messages.Create(t.Context(), workerSess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "hello"}, message.TextContent{Text: "hidden", Hidden: true}},
		})
		require.NoError(t, err)
		ids[i] = msg.ID
	}
	_, err = conn.ExecContext(t.Context(), "UPDATE messages SET created_at = 100 WHERE session_id = ?", workerSess.ID)
	require.NoError(t, err)
	request := func(workerID, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID+"/agent/sessions/"+rootSess.ID+"/workers/"+workerID+"/messages"+query, nil)
		req.SetPathValue("id", workspaceID)
		req.SetPathValue("sid", rootSess.ID)
		req.SetPathValue("worker", workerID)
		rec := httptest.NewRecorder()
		c.handleAgentWorkerMessages(rec, req)
		return rec
	}

	legacy := request("worker-1", "")
	require.Equal(t, http.StatusOK, legacy.Code)
	var legacyBody struct {
		Messages []proto.Message `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(legacy.Body.Bytes(), &legacyBody))
	require.Len(t, legacyBody.Messages, 9)

	var page proto.MessagePage
	latest := request("worker-1", "?limit=3")
	require.Equal(t, http.StatusOK, latest.Code)
	require.NoError(t, json.Unmarshal(latest.Body.Bytes(), &page))
	require.Len(t, page.Messages, 3)
	require.Equal(t, ids[6], page.Before)
	require.True(t, page.HasBefore)
	require.False(t, page.HasAfter)

	backward := request("worker-1", "?limit=3&before="+ids[6])
	require.Equal(t, http.StatusOK, backward.Code)
	require.NoError(t, json.Unmarshal(backward.Body.Bytes(), &page))
	require.Len(t, page.Messages, 3)
	require.Equal(t, ids[3], page.Before)
	require.Equal(t, ids[5], page.After)
	require.True(t, page.HasBefore)
	require.True(t, page.HasAfter)

	forward := request("worker-1", "?limit=3&after="+ids[2])
	require.Equal(t, http.StatusOK, forward.Code)
	require.NoError(t, json.Unmarshal(forward.Body.Bytes(), &page))
	require.Equal(t, ids[3], page.Before)
	require.Equal(t, ids[5], page.After)
	require.True(t, page.HasBefore)
	require.True(t, page.HasAfter)

	center := request("worker-1", "?limit=3&around="+ids[4])
	require.Equal(t, http.StatusOK, center.Code)
	require.NoError(t, json.Unmarshal(center.Body.Bytes(), &page))
	require.Len(t, page.Messages, 3)
	require.Equal(t, ids[4], page.Messages[1].ID)

	textOnly := request("worker-1", "?text_only=true&limit=3")
	require.Equal(t, http.StatusOK, textOnly.Code)
	require.NoError(t, json.Unmarshal(textOnly.Body.Bytes(), &page))
	require.Len(t, page.Messages, 3)

	for _, query := range []string{
		"?limit=0",
		"?limit=201",
		"?limit=abc",
		"?limit=3&before=missing",
		"?before=" + ids[0] + "&after=" + ids[1],
		"?text_only=true&after=" + ids[0],
		"?text_only=true&around=" + ids[0],
	} {
		require.Equal(t, http.StatusBadRequest, request("worker-1", query).Code, query)
	}

	foreignMessage, err := ws.Messages.Create(t.Context(), rootSess.ID, message.CreateMessageParams{Role: message.User})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, request("worker-1", "?limit=3&around="+foreignMessage.ID).Code)
	require.Equal(t, http.StatusNotFound, request("unknown-worker", "?limit=3").Code)
}
