package server

import (
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
