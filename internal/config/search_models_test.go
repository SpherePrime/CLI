package config

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func webSearchModel(id string, enabled bool) catwalk.Model {
	return catwalk.Model{
		ID:      id,
		Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"web_search": enabled}},
	}
}

func TestIsWebSearchModel(t *testing.T) {
	t.Parallel()

	require.False(t, IsWebSearchModel(catwalk.Model{ID: "plain"}))
	require.False(t, IsWebSearchModel(webSearchModel("disabled", false)))
	require.True(t, IsWebSearchModel(webSearchModel("search-enabled", true)))
	require.False(t, IsWebSearchModel(catwalk.Model{
		ID:      "non-bool",
		Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"web_search": "true"}},
	}))
}

func TestProviderWebSearchModelSelection(t *testing.T) {
	t.Parallel()

	t.Run("prefers large provider then small then lowest id", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "relay", Model: "relay/chat"},
				SelectedModelTypeSmall: {Provider: "small-prov", Model: "small/search"},
			},
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"alpha":      {Type: catwalk.TypeOpenAICompat, BaseURL: "https://alpha.example/v1", APIKey: "a", Models: []catwalk.Model{webSearchModel("alpha/search", true)}},
				"small-prov": {Type: catwalk.TypeOpenAI, BaseURL: "https://small.example/v1", APIKey: "s", Models: []catwalk.Model{webSearchModel("small/search", true)}},
				"relay":      {Type: catwalk.TypeOpenAICompat, BaseURL: "https://relay.example/v1/", APIKey: "$RELAY", Models: []catwalk.Model{{ID: "relay/chat"}, webSearchModel("relay/search", true)}},
				"disabled":   {Type: catwalk.TypeOpenAICompat, BaseURL: "https://off.example/v1", Disable: true, Models: []catwalk.Model{webSearchModel("off/search", true)}},
				"anthropic":  {Type: catwalk.TypeAnthropic, BaseURL: "https://anthropic.example/v1", Models: []catwalk.Model{webSearchModel("claude-search", true)}},
				"no-url":     {Type: catwalk.TypeOpenAICompat, BaseURL: "   ", Models: []catwalk.Model{webSearchModel("nourl/search", true)}},
			}),
		}

		// Prefers Large provider.
		found, ok := cfg.ProviderWebSearchModel()
		require.True(t, ok)
		require.Equal(t, ProviderWebSearchModel{
			ProviderID: "relay",
			BaseURL:    "https://relay.example/v1",
			APIKey:     "$RELAY",
			Model:      "relay/search",
		}, found)

		// Falls back to Small provider when Large has no web search model.
		cfg.Models[SelectedModelTypeLarge] = SelectedModel{Provider: "missing", Model: "none"}
		found, ok = cfg.ProviderWebSearchModel()
		require.True(t, ok)
		require.Equal(t, "small-prov", found.ProviderID)
		require.Equal(t, "small/search", found.Model)

		// Falls back to deterministic lowest providerID when neither Large nor Small matches.
		cfg.Models = nil
		found, ok = cfg.ProviderWebSearchModel()
		require.True(t, ok)
		require.Equal(t, "alpha", found.ProviderID)
		require.Equal(t, "alpha/search", found.Model)
	})

	t.Run("ignores non-openai, disabled, and empty baseurl providers", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"disabled":  {Type: catwalk.TypeOpenAICompat, BaseURL: "https://off.example/v1", Disable: true, Models: []catwalk.Model{webSearchModel("off/search", true)}},
				"anthropic": {Type: catwalk.TypeAnthropic, BaseURL: "https://anthropic.example/v1", Models: []catwalk.Model{webSearchModel("claude-search", true)}},
				"no-url":    {Type: catwalk.TypeOpenAICompat, BaseURL: "", Models: []catwalk.Model{webSearchModel("nourl/search", true)}},
				"plain":     {Type: catwalk.TypeOpenAICompat, BaseURL: "https://plain.example/v1", Models: []catwalk.Model{{ID: "chat"}}},
			}),
		}
		_, ok := cfg.ProviderWebSearchModel()
		require.False(t, ok)
	})

	t.Run("prefers small model when large and small share provider and large lacks web search", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "relay", Model: "relay/large-no-search"},
				SelectedModelTypeSmall: {Provider: "relay", Model: "relay/small-search"},
			},
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"relay": {
					Type:    catwalk.TypeOpenAICompat,
					BaseURL: "https://relay.example/v1",
					APIKey:  "k",
					Models: []catwalk.Model{
						webSearchModel("relay/first-search", true),
						{ID: "relay/large-no-search"},
						webSearchModel("relay/small-search", true),
					},
				},
			}),
		}

		found, ok := cfg.ProviderWebSearchModel()
		require.True(t, ok)
		require.Equal(t, "relay", found.ProviderID)
		require.Equal(t, "relay/small-search", found.Model)
	})

	t.Run("nil config returns false", func(t *testing.T) {
		t.Parallel()
		var nilCfg *Config
		_, ok := nilCfg.ProviderWebSearchModel()
		require.False(t, ok)
	})
}
