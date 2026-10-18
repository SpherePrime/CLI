package proto

import (
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPermissionGrantAcceptsStringParamsForMCPTools(t *testing.T) {
	t.Parallel()

	var grant PermissionGrant
	err := json.Unmarshal([]byte(`{"permission":{"id":"p1","tool_name":"mcp_playwright_browser_navigate","params":"{\"url\":\"file:///C:/test\"}"},"action":"allow"}`), &grant)
	require.NoError(t, err)
	require.Equal(t, "p1", grant.Permission.ID)
	require.Equal(t, `{"url":"file:///C:/test"}`, grant.Permission.Params)
}
