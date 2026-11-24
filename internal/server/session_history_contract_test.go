package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/app"
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/history"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type memorySessions struct {
	session.Service
	items map[string]session.Session
}

func (m *memorySessions) Get(_ context.Context, id string) (session.Session, error) {
	s, ok := m.items[id]
	if !ok {
		return session.Session{}, errors.New("not found")
	}
	return s, nil
}

func (m *memorySessions) Save(_ context.Context, s session.Session) (session.Session, error) {
	m.items[s.ID] = s
	return s, nil
}

type memoryHistory struct {
	history.Service
	files map[string][]history.File
}

func (m *memoryHistory) ListBySession(_ context.Context, sessionID string) ([]history.File, error) {
	return m.files[sessionID], nil
}

func buildContractController(t *testing.T, sessions *memorySessions, hist *memoryHistory) (*controllerV1, string) {
	t.Helper()
	b := backend.New(context.Background(), nil, nil)
	wsID := uuid.New().String()
	a := &app.App{
		Sessions: sessions,
		History:  hist,
	}
	ws := &backend.Workspace{
		ID:   wsID,
		Path: t.TempDir(),
		App:  a,
	}
	backend.InsertWorkspaceForTest(b, ws)
	s := &Server{backend: b}
	return &controllerV1{backend: b, server: s}, wsID
}

func TestPutWorkspaceSession_PreservesSnakeCaseAndConcurrentUsage(t *testing.T) {
	t.Parallel()
	const sid = "sess-1"
	sessions := &memorySessions{
		items: map[string]session.Session{
			sid: {
				ID:               sid,
				Title:            "Original",
				PromptTokens:     1200,
				CompletionTokens: 450,
				SummaryMessageID: "sum-42",
				Cost:             0.25,
				Todos:            []session.Todo{{Content: "task", Status: session.TodoStatusPending}},
			},
		},
	}
	c, wsID := buildContractController(t, sessions, &memoryHistory{})

	body := `{"id":"wrong-id","title":"Renamed","prompt_tokens":100,"completion_tokens":50,"summary_message_id":""}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/workspaces/"+wsID+"/sessions/"+sid, strings.NewReader(body))
	req.SetPathValue("id", wsID)
	req.SetPathValue("sid", sid)
	rec := httptest.NewRecorder()
	c.handlePutWorkspaceSession(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, sid, got.ID)
	require.Equal(t, "Renamed", got.Title)
	require.Equal(t, int64(1200), got.PromptTokens)
	require.Equal(t, int64(450), got.CompletionTokens)
	require.Equal(t, "sum-42", got.SummaryMessageID)
	require.InDelta(t, 0.25, got.Cost, 1e-9)
	require.Len(t, got.Todos, 1)

	titleOnly := `{"title":"Title Only"}`
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/workspaces/"+wsID+"/sessions/"+sid, strings.NewReader(titleOnly))
	req2.SetPathValue("id", wsID)
	req2.SetPathValue("sid", sid)
	rec2 := httptest.NewRecorder()
	c.handlePutWorkspaceSession(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var got2 proto.Session
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &got2))
	require.Equal(t, sid, got2.ID)
	require.Equal(t, "Title Only", got2.Title)
	require.Equal(t, int64(1200), got2.PromptTokens)
	require.Equal(t, int64(450), got2.CompletionTokens)
	require.Equal(t, "sum-42", got2.SummaryMessageID)

	higher := `{"title":"Higher Usage","prompt_tokens":2000,"completion_tokens":900,"summary_message_id":"sum-99","cost":0.75}`
	req3 := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/workspaces/"+wsID+"/sessions/"+sid, strings.NewReader(higher))
	req3.SetPathValue("id", wsID)
	req3.SetPathValue("sid", sid)
	rec3 := httptest.NewRecorder()
	c.handlePutWorkspaceSession(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)

	var got3 proto.Session
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &got3))
	require.Equal(t, int64(2000), got3.PromptTokens)
	require.Equal(t, int64(900), got3.CompletionTokens)
	require.Equal(t, "sum-99", got3.SummaryMessageID)
	require.InDelta(t, 0.75, got3.Cost, 1e-9)
}

func TestGetWorkspaceSessionHistory_SnakeCaseJSON(t *testing.T) {
	t.Parallel()
	const sid = "sess-hist"
	hist := &memoryHistory{
		files: map[string][]history.File{
			sid: {
				{
					ID:        "f-1",
					SessionID: sid,
					Path:      "main.go",
					Content:   "package main",
					Version:   2,
					CreatedAt: 100,
					UpdatedAt: 200,
				},
			},
		},
	}
	c, wsID := buildContractController(t, &memorySessions{items: map[string]session.Session{}}, hist)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/sessions/"+sid+"/history", nil)
	req.SetPathValue("id", wsID)
	req.SetPathValue("sid", sid)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceSessionHistory(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var raw []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Len(t, raw, 1)
	require.Equal(t, "f-1", raw[0]["id"])
	require.Equal(t, sid, raw[0]["session_id"])
	require.Equal(t, "main.go", raw[0]["path"])
	require.Equal(t, "package main", raw[0]["content"])
	require.Equal(t, float64(2), raw[0]["version"])
	require.Equal(t, float64(100), raw[0]["created_at"])
	require.Equal(t, float64(200), raw[0]["updated_at"])
}

func TestGetWorkspaceChannelMessages_ReturnsEmptyArray(t *testing.T) {
	t.Parallel()
	c, wsID := buildContractController(t, &memorySessions{items: map[string]session.Session{}}, &memoryHistory{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/"+wsID+"/channel-messages", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceChannelMessages(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]", strings.TrimSpace(rec.Body.String()))

	reqMissing := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/workspaces/missing/channel-messages", nil)
	reqMissing.SetPathValue("id", "missing")
	recMissing := httptest.NewRecorder()
	c.handleGetWorkspaceChannelMessages(recMissing, reqMissing)
	require.Equal(t, http.StatusNotFound, recMissing.Code)
}

func TestProviderSaveUnknownWorkspaceReturns404(t *testing.T) {
	t.Parallel()
	c, _ := buildContractController(t, &memorySessions{items: map[string]session.Session{}}, &memoryHistory{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/missing/config/provider-save", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceProviderSave(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRunQueuedNowUsesJSONErrorFormat(t *testing.T) {
	t.Parallel()
	c, wsID := buildContractController(t, &memorySessions{items: map[string]session.Session{}}, &memoryHistory{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/"+wsID+"/agent/sessions/s1/prompts/run-now", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", wsID)
	req.SetPathValue("sid", "s1")
	rec := httptest.NewRecorder()
	c.handleRunQueuedNow(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp proto.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	require.Equal(t, "queued prompt id is required", errResp.Message)
}

func TestTruncateTailUTF8PreservesRuneBoundaries(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("Ж", 50)
	truncated := truncateTailUTF8(input, 15)
	require.True(t, utf8.ValidString(truncated))
	require.LessOrEqual(t, len(truncated), 15)
	require.Equal(t, strings.Repeat("Ж", 7), truncated)
}
