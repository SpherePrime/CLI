package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type sttHarness struct {
	*e2eHarness
	workspace *backend.Workspace
}

func newSttHarness(t *testing.T) *sttHarness {
	t.Helper()
	projectDir := t.TempDir()
	dataDir := t.TempDir()
	h := &sttHarness{e2eHarness: newRealCreateHarness(t)}
	resp := h.postWorkspace(t, proto.Workspace{Path: projectDir, DataDir: dataDir, ClientID: uuid.New().String()})
	ws, err := h.backend.GetWorkspace(resp.ID)
	require.NoError(t, err)
	resolvedDataDir := ws.Cfg.Config().Options.DataDirectory
	backend.SetWorkspaceShutdownFnForTest(ws, func() { _ = db.Release(resolvedDataDir) })
	t.Cleanup(h.backend.CloseInProcess)
	h.workspace = ws
	return h
}

func (h *sttHarness) get(t *testing.T, path string, out any) int {
	t.Helper()
	resp, err := http.Get(h.httpSrv.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	return resp.StatusCode
}

func (h *sttHarness) post(t *testing.T, path string, body any, out any) int {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(h.httpSrv.URL+path, "application/json", bytes.NewReader(raw))
	require.NoError(t, err)
	defer resp.Body.Close()
	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}

func TestSttCatalogAndSelectLanguage(t *testing.T) {
	h := newSttHarness(t)
	base := "/v1/workspaces/" + h.workspace.ID + "/stt"

	var catalog proto.SttCatalog
	require.Equal(t, http.StatusOK, h.get(t, base, &catalog))
	require.Equal(t, voice.EngineTinyASR, catalog.Engine)
	require.Equal(t, "en-ru-uk", catalog.Model)
	require.Equal(t, "ru", catalog.Language)
	require.Equal(t, voice.AllowedLanguages, catalog.Languages)

	var selection proto.SttSelection
	require.Equal(t, http.StatusOK, h.post(t, base+"/select", proto.SttModelRequest{Language: "ru"}, &selection))
	require.Equal(t, proto.SttSelection{Engine: voice.EngineTinyASR, Model: "en-ru-uk", Language: "ru"}, selection)

	require.Equal(t, http.StatusOK, h.get(t, base, &catalog))
	require.Equal(t, "ru", catalog.Language)

	require.Equal(t, http.StatusOK, h.post(t, base+"/select", proto.SttModelRequest{Language: "auto"}, &selection))
	require.Equal(t, "auto", selection.Language)

	require.Equal(t, http.StatusOK, h.post(t, base+"/select", proto.SttModelRequest{Language: "unsupported"}, &selection))
	require.Equal(t, "ru", selection.Language)
}
