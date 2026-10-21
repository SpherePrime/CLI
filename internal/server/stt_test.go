package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// This file drives the /v1/workspaces/{id}/stt endpoints against a real
// workspace, following the e2e harness pattern: isolated HOME so the
// config store writes to a temp tree, and a workspace created through the
// real HTTP path so config writes and the event broker behave like
// production.

// sttHarness is a real server plus a real workspace to exercise the STT
// endpoints with.
type sttHarness struct {
	*e2eHarness
	workspace *backend.Workspace
}

func newSttHarness(t *testing.T) *sttHarness {
	t.Helper()
	h := &sttHarness{e2eHarness: newRealCreateHarness(t)}
	resp := h.postWorkspace(t, proto.Workspace{Path: t.TempDir(), DataDir: t.TempDir(), ClientID: uuid.New().String()})
	ws, err := h.backend.GetWorkspace(resp.ID)
	require.NoError(t, err)
	// Skip the full app teardown in cleanup, but release the pooled DB
	// connection so the temp data directory can be removed on Windows.
	dataDir := ws.Cfg.Config().Options.DataDirectory
	backend.SetWorkspaceShutdownFnForTest(ws, func() { _ = db.Release(dataDir) })
	h.workspace = ws
	return h
}

// get issues a GET and decodes the JSON body into out.
func (h *sttHarness) get(t *testing.T, path string, out any) int {
	t.Helper()
	resp, err := http.Get(h.httpSrv.URL + path) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	return resp.StatusCode
}

// post issues a POST with a JSON body and decodes the response into out.
func (h *sttHarness) post(t *testing.T, path, body string, out any) int {
	t.Helper()
	resp, err := http.Post(h.httpSrv.URL+path, "application/json", strings.NewReader(body)) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()
	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}

// providerByID indexes the catalog by provider id.
func providerByID(t *testing.T, catalog proto.SttCatalog, id string) proto.SttProvider {
	t.Helper()
	for _, provider := range catalog.Providers {
		if provider.ID == id {
			return provider
		}
	}
	t.Fatalf("catalog has no %s provider", id)
	return proto.SttProvider{}
}

// waitForTaskState polls the task list until the named task reaches state.
func (h *sttHarness) waitForTaskState(t *testing.T, taskName, state string) proto.PluginTask {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var tasks []proto.PluginTask
		code := h.get(t, "/v1/workspaces/"+h.workspace.ID+"/stt/tasks", &tasks)
		require.Equal(t, 200, code)
		for _, task := range tasks {
			if task.Name == taskName {
				if task.State == state {
					return task
				}
				if state != "failed" && task.State == "failed" {
					t.Fatalf("task %s failed: %s", taskName, task.Error)
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s never reached %s", taskName, state)
	return proto.PluginTask{}
}

// TestSttCatalogRouteShape checks the catalog contract: three providers,
// google with no models, and the configured engine echoed back.
func TestSttCatalogRouteShape(t *testing.T) {
	h := newSttHarness(t)

	var catalog proto.SttCatalog
	require.Equal(t, 200, h.get(t, "/v1/workspaces/"+h.workspace.ID+"/stt", &catalog))
	require.Empty(t, catalog.Engine)
	require.Len(t, catalog.Providers, 3)

	google := providerByID(t, catalog, "google")
	require.Equal(t, "Google STT", google.Title)
	require.True(t, google.Available)
	require.Empty(t, google.Models)

	vosk := providerByID(t, catalog, "vosk")
	require.True(t, vosk.Available)
	require.NotEmpty(t, vosk.Models)
	for _, model := range vosk.Models {
		require.NotEmpty(t, model.ID)
		require.Greater(t, model.SizeBytes, int64(0))
		require.False(t, model.Installed)
		require.False(t, model.Active)
	}
}

// TestSttSelectPersistsInConfig writes the choice through the endpoint and
// verifies it lands in the config and shows up in the catalog.
func TestSttSelectPersistsInConfig(t *testing.T) {
	h := newSttHarness(t)
	id := h.workspace.ID

	var selection proto.SttSelection
	code := h.post(t, "/v1/workspaces/"+id+"/stt/select",
		`{"provider":"whisper","model":"ggml-tiny"}`, &selection)
	require.Equal(t, 200, code)
	require.Equal(t, proto.SttSelection{Engine: "whisper", Model: "ggml-tiny"}, selection)

	// The write must survive in the config store, not just the response.
	options := h.workspace.Cfg.Config().Options.Voice
	require.NotNil(t, options)
	require.Equal(t, "whisper", options.Engine)
	require.Equal(t, "ggml-tiny", options.Model)

	var catalog proto.SttCatalog
	require.Equal(t, 200, h.get(t, "/v1/workspaces/"+id+"/stt", &catalog))
	require.Equal(t, "whisper", catalog.Engine)
	require.Equal(t, "ggml-tiny", catalog.Model)
	whisper := providerByID(t, catalog, "whisper")
	for _, model := range whisper.Models {
		// Active follows the configured model id, installed or not.
		require.Equal(t, model.ID == "ggml-tiny", model.Active, model.ID)
	}

	// Selecting the hosted engine clears the model again.
	code = h.post(t, "/v1/workspaces/"+id+"/stt/select", `{"provider":"google","model":""}`, &selection)
	require.Equal(t, 200, code)
	require.Equal(t, proto.SttSelection{Engine: "google", Model: ""}, selection)
	require.Equal(t, 400, h.post(t, "/v1/workspaces/"+id+"/stt/select", `{"provider":"nope","model":""}`, nil))
}

// sttFakeDownloads serves tiny stand-in archives for the model installer,
// with an optional gate to hold a download open for the conflict test.
func sttFakeDownloads(t *testing.T, holdVoskRuntime <-chan struct{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/vosk-runtime/"):
			if holdVoskRuntime != nil {
				select {
				case <-holdVoskRuntime:
				case <-r.Context().Done():
					return
				}
			}
			writeTestZip(t, w, map[string][]byte{
				"libvosk.so":    []byte("elf"),
				"vosk.dll":      []byte("mz"),
				"libvosk.dylib": []byte("mach-o"),
			})
		case strings.Contains(r.URL.Path, "/vosk-models/"):
			name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
			writeTestZip(t, w, map[string][]byte{
				name + "/conf/model.conf": []byte("conf"),
				name + "/am/final.mdl":    []byte("model"),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// writeTestZip sends files as a zip archive.
func writeTestZip(t *testing.T, w http.ResponseWriter, files map[string][]byte) {
	t.Helper()
	buf := &bytes.Buffer{}
	writer := zip.NewWriter(buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	w.Header().Set("Content-Type", "application/zip")
	_, _ = w.Write(buf.Bytes())
}

// TestSttInstallTaskCompletes drives a full install through the endpoints
// against a fake download server: the task appears, reports progress and
// finishes with the model marked installed.
func TestSttInstallTaskCompletes(t *testing.T) {
	h := newSttHarness(t)
	voice.SetSttDownloadRewritesForTest(map[string]string{
		"https://alphacephei.com/vosk/models/":                            sttFakeDownloads(t, nil).URL + "/vosk-models/",
		"https://github.com/alphacep/vosk-api/releases/download/v0.3.45/": sttFakeDownloads(t, nil).URL + "/vosk-runtime/",
	})
	t.Cleanup(func() { voice.SetSttDownloadRewritesForTest(map[string]string{}) })

	id := h.workspace.ID
	var started proto.Started
	code := h.post(t, "/v1/workspaces/"+id+"/stt/install",
		`{"provider":"vosk","model":"vosk-model-small-ru-0.22"}`, &started)
	require.Equal(t, 200, code)
	require.True(t, started.Started)

	taskName := "stt:vosk:vosk-model-small-ru-0.22"
	task := h.waitForTaskState(t, taskName, "completed")
	require.NotZero(t, task.Done)

	var catalog proto.SttCatalog
	require.Equal(t, 200, h.get(t, "/v1/workspaces/"+id+"/stt", &catalog))
	require.True(t, providerByID(t, catalog, "vosk").Models[0].Installed)

	// The task list belongs to STT only; the plugin task list stays clean.
	var pluginTasks []proto.PluginTask
	code = h.get(t, "/v1/workspaces/"+id+"/plugins/tasks", &pluginTasks)
	require.Equal(t, 200, code)
	for _, task := range pluginTasks {
		require.NotEqual(t, taskName, task.Name)
	}
}

// TestSttInstallConflictAndRemove covers the duplicate-install 409 and the
// remove endpoint clearing files and cancelling a running install.
func TestSttInstallConflictAndRemove(t *testing.T) {
	h := newSttHarness(t)
	release := make(chan struct{})
	voice.SetSttDownloadRewritesForTest(map[string]string{
		"https://alphacephei.com/vosk/models/":                            sttFakeDownloads(t, nil).URL + "/vosk-models/",
		"https://github.com/alphacep/vosk-api/releases/download/v0.3.45/": sttFakeDownloads(t, release).URL + "/vosk-runtime/",
	})
	t.Cleanup(func() {
		close(release)
		voice.SetSttDownloadRewritesForTest(map[string]string{})
	})

	id := h.workspace.ID
	taskName := "stt:vosk:vosk-model-small-ru-0.22"
	var started proto.Started
	code := h.post(t, "/v1/workspaces/"+id+"/stt/install",
		`{"provider":"vosk","model":"vosk-model-small-ru-0.22"}`, &started)
	require.Equal(t, 200, code)
	// Wait until the install is parked in the blocked download...
	h.waitForTaskState(t, taskName, "running")
	// ...so the second request hits the conflict.
	require.Equal(t, 409, h.post(t, "/v1/workspaces/"+id+"/stt/install",
		`{"provider":"vosk","model":"vosk-model-small-ru-0.22"}`, nil))

	// Removing cancels the running install and clears any files.
	require.Equal(t, 200, h.post(t, "/v1/workspaces/"+id+"/stt/remove",
		`{"provider":"vosk","model":"vosk-model-small-ru-0.22"}`, nil))

	var catalog proto.SttCatalog
	require.Equal(t, 200, h.get(t, "/v1/workspaces/"+id+"/stt", &catalog))
	require.False(t, providerByID(t, catalog, "vosk").Models[0].Installed)
}

// TestSttRoutesRejectUnknownWorkspace mirrors the plugins route check for
// the STT routes.
func TestSttRoutesRejectUnknownWorkspace(t *testing.T) {
	s := NewServer(nil, "tcp", "127.0.0.1:0")
	for _, path := range []string{"stt", "stt/tasks"} {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/workspaces/missing/"+path, nil))
		if r.Code != 404 {
			t.Errorf("%s: status %d", path, r.Code)
		}
	}
	for _, path := range []string{"stt/install", "stt/remove", "stt/select"} {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest("POST", "/v1/workspaces/missing/"+path,
			strings.NewReader(`{"provider":"vosk","model":"x"}`)))
		if r.Code != 404 {
			t.Errorf("%s: status %d", path, r.Code)
		}
	}
}
