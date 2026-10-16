package server

import (
	"context"
	"encoding/json"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/question"
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
	if !capabilities.RunRegistry || !capabilities.PendingRequests || capabilities.EventReplay || capabilities.RunRegistryPersistence != "workspace_lifetime" {
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
