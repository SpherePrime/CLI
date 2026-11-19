package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPostWorkspaceConfigGameDev_HTTP(t *testing.T) {
	srv, ws := newOrchestratorServerTestHarness(t)

	post := func(wsID string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/workspaces/"+wsID+"/config/gamedev",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec
	}

	require.False(t, ws.Cfg.IsGameDevEnabled(""))
	require.False(t, ws.Cfg.IsGameDevEnabled("chat-1"))

	payload, err := json.Marshal(proto.ConfigGameDevRequest{
		Scope:   config.ScopeGlobal,
		Enabled: true,
	})
	require.NoError(t, err)
	rec := post(ws.ID, payload)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, ws.Cfg.IsGameDevEnabled(""))
	require.True(t, ws.Cfg.IsGameDevEnabled("chat-2"))

	sessPayload, err := json.Marshal(proto.ConfigGameDevRequest{
		SessionID: "chat-1",
		Enabled:   false,
	})
	require.NoError(t, err)
	rec = post(ws.ID, sessPayload)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, ws.Cfg.IsGameDevEnabled("chat-1"))
	require.True(t, ws.Cfg.IsGameDevEnabled("chat-2"))

	rec = post(ws.ID, []byte("not-json"))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = post("no-such-workspace", payload)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
