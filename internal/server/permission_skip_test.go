package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPermissionSkipSessionHTTP(t *testing.T) {
	c, workspaceID := buildAgentWorkspace(t, nil)
	workspace, err := c.backend.GetWorkspace(workspaceID)
	require.NoError(t, err)
	workspace.Permissions = permission.NewPermissionService(t.TempDir(), false, nil)

	setSkip := func(body string) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		request.SetPathValue("id", workspaceID)
		response := httptest.NewRecorder()
		c.handlePostWorkspacePermissionsSkip(response, request)
		require.Equal(t, http.StatusOK, response.Code)
	}
	getSkip := func(sessionID string, expected bool) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?session_id="+sessionID, nil)
		request.SetPathValue("id", workspaceID)
		response := httptest.NewRecorder()
		c.handleGetWorkspacePermissionsSkip(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		var result proto.PermissionSkipRequest
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		require.Equal(t, expected, result.Skip)
		require.Equal(t, sessionID, result.SessionID)
	}

	setSkip(`{"skip":true,"session_id":"full"}`)
	getSkip("full", true)
	getSkip("ask", false)
	getSkip("", false)
	setSkip(`{"skip":false,"session_id":"ask"}`)
	setSkip(`{"skip":true}`)
	getSkip("ask", false)
	getSkip("other", true)
	getSkip("", true)
}
