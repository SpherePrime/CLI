package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopPluginRoutesRejectUnknownWorkspace(t *testing.T) {
	s := NewServer(nil, "tcp", "127.0.0.1:0")
	for _, path := range []string{"plugins", "plugins/tasks", "commands", "jobs", "pending", "runs", "models/capabilities"} {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/workspaces/missing/"+path, nil))
		if r.Code != 404 {
			t.Errorf("%s: status %d", path, r.Code)
		}
	}
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("POST", "/v1/workspaces/missing/voice/transcribe", strings.NewReader(`{"mime_type":"audio/webm","audio_base64":"AA=="}`)))
	if r.Code != 404 {
		t.Fatalf("voice status %d", r.Code)
	}
}
