package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/question"
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCapabilitiesDeclareRecoveryLimitations(t *testing.T) {
	s := NewServer(nil, "tcp", "127.0.0.1:0")
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/capabilities", nil))
	if r.Code != 200 {
		t.Fatalf("status %d", r.Code)
	}
	var capabilities proto.DesktopCapabilities
	if err := json.Unmarshal(r.Body.Bytes(), &capabilities); err != nil {
		t.Fatal(err)
	}
	if !capabilities.RunRegistry || !capabilities.PendingRequests || capabilities.EventReplay || capabilities.RunRegistryPersistence != "sqlite" {
		t.Fatalf("invalid capabilities: %+v", capabilities)
	}
}

func TestPendingSnapshotRecoversQuestionWithoutSSE(t *testing.T) {
	h := newE2EHarness(t)
	service := question.NewService()
	h.app.Questions = service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := service.Subscribe(ctx)
	go service.Ask(ctx, question.Request{ID: "pending-batch", SessionID: "session", Questions: []question.Question{{ID: "q", Type: question.TypeFreeText, Text: "Name?", Description: "Your name"}}})
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("question did not start")
	}
	r := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/workspaces/"+h.workspace.ID+"/pending", nil))
	var pending struct {
		Question *question.Request `json:"question"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || pending.Question == nil || pending.Question.ID != "pending-batch" {
		t.Fatalf("bad snapshot %s", r.Body.String())
	}
}

func TestJobsSnapshotFiltersOriginatingSessionAndWorkspace(t *testing.T) {
	h := newE2EHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := shell.GetBackgroundShellManager()
	for _, owner := range []shell.BackgroundOwnership{{WorkspacePath: h.workspace.Path, SessionID: "selected"}, {WorkspacePath: h.workspace.Path, SessionID: "another"}, {WorkspacePath: t.TempDir(), SessionID: "selected"}} {
		job, err := manager.Start(ctx, t.TempDir(), nil, "echo fixture", "fixture", owner)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { manager.Kill(job.ID) })
	}
	r := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/workspaces/"+h.workspace.ID+"/jobs?session_id=selected", nil))
	var jobs []proto.BackgroundJob
	if err := json.Unmarshal(r.Body.Bytes(), &jobs); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || len(jobs) != 1 || jobs[0].SessionID != "selected" {
		t.Fatalf("wrong jobs snapshot: %s", r.Body.String())
	}
}

func TestModelCapabilitiesIncludesPrimeAISubscriptionMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("PRIME_GLOBAL_DATA", tmpDir)

	var polled atomic.Bool
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/"):
			polled.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "acct_srv_test",
				"user":   map[string]string{"email": "caps@sphereprime.ru"},
			})
		case r.URL.Path == "/api/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "caps@sphereprime.ru"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer authSrv.Close()

	primeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/prime/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_srv",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/prime/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{
						"id":                 "gpt-5-pro",
						"name":               "GPT-5 Pro",
						"kind":               "text",
						"planLabel":          "Pro",
						"locked":             false,
						"allowedTariffNames": []string{"Pro", "Ultra"},
					},
					{
						"id":                 "opus-ultra",
						"name":               "Opus Ultra",
						"kind":               "text",
						"planLabel":          "Ultra",
						"locked":             true,
						"allowedTariffNames": []string{"Ultra"},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer primeSrv.Close()
	t.Setenv("PRIMEAI_URL", primeSrv.URL)

	svc := account.NewService(
		account.WithBaseURL(authSrv.URL),
		account.WithOpenBrowser(func(string) error { return nil }),
		account.WithPollInterval(5*time.Millisecond),
		account.WithMaxAttempts(20),
	)
	defer svc.Close()

	_, err := svc.StartLogin(t.Context())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return polled.Load() && svc.Token() == "acct_srv_test"
	}, 2*time.Second, 10*time.Millisecond)

	h := newE2EHarness(t)
	h.srv.account = svc

	cfg, err := config.Init(h.workspace.Path, tmpDir, false)
	require.NoError(t, err)
	h.workspace.Cfg = cfg

	rec := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/v1/workspaces/"+h.workspace.ID+"/models/capabilities", nil))
	require.Equal(t, 200, rec.Code)

	var caps []proto.ModelCapabilities
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &caps))

	byID := map[string]proto.ModelCapabilities{}
	for _, item := range caps {
		if item.ProviderID == primeai.Name {
			byID[item.ModelID] = item
		}
	}

	pro, ok := byID["gpt-5-pro"]
	require.True(t, ok)
	require.Equal(t, "Pro", pro.PlanLabel)
	require.False(t, pro.Locked)
	require.Equal(t, []string{"Pro", "Ultra"}, pro.AllowedTariffs)

	ultra, ok := byID["opus-ultra"]
	require.True(t, ok)
	require.Equal(t, "Ultra", ultra.PlanLabel)
	require.True(t, ultra.Locked)
	require.Equal(t, []string{"Ultra"}, ultra.AllowedTariffs)
}
