package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSmartToolsOutgoingCatalog(t *testing.T) {
	for _, smart := range []bool{false, true} {
		t.Run(fmt.Sprint(smart), func(t *testing.T) {
			var catalogs [][]string
			var mu sync.Mutex
			requestIndex := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				var body struct {
					Tools []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				var names []string
				for _, tool := range body.Tools {
					names = append(names, tool.Function.Name)
				}
				catalogs = append(catalogs, names)
				w.Header().Set("Content-Type", "text/event-stream")
				var delta any = map[string]any{"content": "done"}
				finish := "stop"
				if smart && requestIndex < 2 {
					name, input := "search_mcp", `{"query":"unique_selected","limit":1}`
					if requestIndex == 1 {
						name, input = "mcp_demo_unique_selected", `{}`
					}
					delta = map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprint(requestIndex), "type": "function", "function": map[string]any{"name": name, "arguments": input}}}}
					finish = "tool_calls"
				}
				requestIndex++
				chunk := map[string]any{"id": "test", "object": "chat.completion.chunk", "model": "mock", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
				payload, _ := json.Marshal(chunk)
				_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
			}))
			defer server.Close()
			provider, err := openaicompat.New(openaicompat.WithBaseURL(server.URL+"/v1"), openaicompat.WithAPIKey("test"))
			require.NoError(t, err)
			model, err := provider.LanguageModel(t.Context(), "mock")
			require.NoError(t, err)
			env := testEnv(t)
			var palette []fantasy.AgentTool
			var entries []tools.SmartSearchEntry
			ran := 0
			for i := range 140 {
				name := fmt.Sprintf("mcp_demo_extra_%03d", i)
				if i == 0 {
					name = "mcp_demo_unique_selected"
				}
				tool := fantasy.NewAgentTool(name, name, func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
					ran++
					return fantasy.NewTextResponse("ok"), nil
				})
				palette = append(palette, tool)
				entries = append(entries, tools.SmartSearchEntry{Kind: "mcp", Name: name, Description: name})
			}
			if smart {
				palette = append(palette, tools.NewSearchMCPTool(entries), tools.NewSearchToolsTool(nil), tools.NewSearchSkillsTool(nil, nil))
			}
			agent := testSessionAgent(env, model, model, "system", palette...)
			session, err := env.sessions.Create(t.Context(), "test")
			require.NoError(t, err)
			_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "find tool"})
			require.NoError(t, err)
			mu.Lock()
			if smart {
				require.Len(t, catalogs, 3)
				require.ElementsMatch(t, []string{"search_mcp", "search_tools", "search_skills"}, catalogs[0])
				for _, catalog := range catalogs[1:] {
					require.Len(t, catalog, 4)
					require.Contains(t, catalog, "mcp_demo_unique_selected")
					require.NotContains(t, catalog, "mcp_demo_extra_001")
				}
				require.Equal(t, 1, ran)
				mu.Unlock()
				_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "new turn"})
				require.NoError(t, err)
				mu.Lock()
				require.ElementsMatch(t, catalogs[0], catalogs[3])
				mu.Unlock()
				agent.SetTools(palette[:140])
				_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "standard mode"})
				require.NoError(t, err)
				mu.Lock()
				require.Len(t, catalogs[4], 140)
				requestIndex = 0
				mu.Unlock()
				agent.SetTools(palette)
				_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "smart mode again"})
				require.NoError(t, err)
				mu.Lock()
				require.ElementsMatch(t, catalogs[0], catalogs[5])
				require.Len(t, catalogs[6], 4)
				require.Equal(t, 2, ran)
			} else {
				require.Len(t, catalogs[0], 140)
				require.Contains(t, catalogs[0], "mcp_demo_extra_001")
			}
			mu.Unlock()
		})
	}
}
