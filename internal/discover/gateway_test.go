package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDiscoverCodexGatewayCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"codex/model","supports_vision":true,"reasoning_efforts":["low","high"],"default_reasoning_effort":"high","gateway":{"type":"codex","unsupported_generation_parameters":["max_tokens","temperature"]}},{"id":"ordinary"}]}`))
	}))
	defer server.Close()
	models, err := DiscoverModels(context.Background(), Config{ID: "test", BaseURL: server.URL}, &mockResolver{})
	require.NoError(t, err)
	require.True(t, models[0].CanReason)
	require.Equal(t, []string{"low", "high"}, models[0].ReasoningLevels)
	require.Equal(t, "high", models[0].DefaultReasoningEffort)
	require.True(t, models[0].SupportsImages)
	require.Equal(t, "codex", models[0].Options.ProviderOptions["gateway"].(map[string]any)["type"])
	require.Nil(t, models[1].Options.ProviderOptions)
	require.False(t, models[1].CanReason)
}

func TestDiscoverGatewayFillsMissingExistingContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"missing","context_window":256000,"gateway":{"type":"codex"}},{"id":"override","context_window":256000,"gateway":{"type":"codex"}},{"id":"ordinary","context_window":256000},{"id":"fallback","max_context_length":256000,"gateway":{"type":"codex"}}]}`))
	}))
	defer server.Close()
	existing := []catwalk.Model{
		{ID: "missing", Name: "Custom", CanReason: true},
		{ID: "override", ContextWindow: 60000},
		{ID: "ordinary"},
		{ID: "fallback"},
	}
	models, err := DiscoverModels(context.Background(), Config{ID: "test", BaseURL: server.URL, ExistingModels: existing}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 4)
	require.Equal(t, int64(256000), models[0].ContextWindow)
	require.Equal(t, "Custom", models[0].Name)
	require.True(t, models[0].CanReason)
	require.Equal(t, int64(60000), models[1].ContextWindow)
	require.Zero(t, models[2].ContextWindow)
	require.Equal(t, int64(256000), models[3].ContextWindow)
	require.Zero(t, existing[0].ContextWindow)
}

func TestDiscoverRemovesOnlyWithdrawnCodexDefaultAlias(t *testing.T) {
	for _, test := range []struct {
		name, catalog    string
		marked, retained bool
	}{
		{"withdrawn", `{"data":[{"id":"codex/model","gateway":{"type":"codex"}}]}`, true, false},
		{"advertised", `{"data":[{"id":"codex-default","gateway":{"type":"codex"}}]}`, true, true},
		{"ordinary existing", `{"data":[{"id":"codex/model","gateway":{"type":"codex"}}]}`, false, true},
		{"ordinary catalog", `{"data":[{"id":"ordinary"}]}`, true, true},
		{"empty catalog", `{"data":[]}`, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(test.catalog))
			}))
			defer server.Close()
			alias := catwalk.Model{ID: "codex-default"}
			if test.marked {
				alias.Options.ProviderOptions = map[string]any{"gateway": map[string]any{"type": "codex"}}
			}
			models, err := DiscoverModels(context.Background(), Config{ID: "test", BaseURL: server.URL, ExistingModels: []catwalk.Model{alias}}, &mockResolver{})
			require.NoError(t, err)
			found := false
			for _, model := range models {
				found = found || model.ID == "codex-default"
			}
			require.Equal(t, test.retained, found)
		})
	}
}
