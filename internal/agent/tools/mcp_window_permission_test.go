package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	sdk "github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWindowMCPUsesNormalChatPermission(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		skip  bool
		allow bool
	}{
		{name: "confirm allows once", allow: true},
		{name: "full access executes without confirmation", skip: true, allow: true},
		{name: "denial never reaches desktop"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			calls := make(chan *sdk.CallToolRequest, 2)
			server := sdk.NewServer(&sdk.Implementation{Name: "window-fixture"}, nil)
			server.AddTool(&sdk.Tool{
				Name:        "window_request_access",
				InputSchema: map[string]any{"type": "object", "additionalProperties": true},
			}, func(_ context.Context, request *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				calls <- request
				return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "granted"}}}, nil
			})
			httpServer := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil))
			defer httpServer.Close()
			name := "window-permission-fixture"
			store := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPHttp, URL: httpServer.URL}}})
			require.NoError(t, mcp.InitializeSingle(ctx, name, store))
			defer mcp.Close(context.Background())
			permissions := permission.NewPermissionService(t.TempDir(), scenario.skip, nil)
			requests := permissions.Subscribe(ctx)
			tool := &Tool{mcpName: name, tool: &mcp.Tool{Name: "window_request_access"}, cfg: store, permissions: permissions, workingDir: t.TempDir()}
			result := make(chan error, 1)
			go func() {
				_, err := tool.Run(context.WithValue(ctx, SessionIDContextKey, "actual-chat"), fantasy.ToolCall{
					ID: "window-call", Name: tool.Name(), Input: `{"hwnd":"123","pid":55,"identity":"native-token","title":"Fixture editor","mode":"control","sessionId":"forged-chat","_meta":{"prime/sessionId":"forged-chat"}}`,
				})
				result <- err
			}()
			if !scenario.skip {
				select {
				case event := <-requests:
					require.Contains(t, event.Payload.Description, "Fixture editor")
					require.Contains(t, event.Payload.Description, "123")
					select {
					case <-calls:
						t.Fatal("desktop executed before approval")
					default:
					}
					if scenario.allow {
						permissions.Grant(event.Payload)
					} else {
						permissions.Deny(event.Payload)
					}
				case <-ctx.Done():
					t.Fatal("chat permission was not requested")
				}
			}
			require.NoError(t, <-result)
			if scenario.allow {
				select {
				case request := <-calls:
					require.Equal(t, "actual-chat", request.Params.Meta["prime/sessionId"])
				case <-ctx.Done():
					t.Fatal("approved call did not reach desktop")
				}
			}
			select {
			case <-requests:
				t.Fatal("unexpected additional approval")
			case <-calls:
				t.Fatal("unexpected desktop execution")
			default:
			}
		})
	}
}
