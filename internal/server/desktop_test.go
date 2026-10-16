package server

import (
	"context"
	"encoding/json"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/question"
	"github.com/SpherePrime/CLI/internal/shell"
	"net/http/httptest"
	"testing"
	"time"
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
