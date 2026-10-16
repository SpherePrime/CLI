package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunQueryPreservesNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws/runs/run" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"run not found"}`))
	}))
	defer server.Close()
	client := captureClient(t, server)
	if _, err := client.GetRun(context.Background(), "ws", "run"); err == nil {
		t.Fatal("missing run was silently returned")
	}
}
