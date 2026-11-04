package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
