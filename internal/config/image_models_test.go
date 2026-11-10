package config

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestIsImageOutputModel(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]bool{
		"antigravity/gemini-3.1-flash-image": true,
		"gpt-image-1":                        true,
		"imagen-4.0-generate":                true,
		"antigravity/gemini-3-flash":         false,
		"claude-opus-5-5":                    false,
		"imagery-analyzer-pro":               false,
	} {
		require.Equal(t, want, IsImageOutputModel(catwalk.Model{ID: id}), id)
	}
	flagged := catwalk.Model{ID: "painter", Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"image_output": true}}}
	require.True(t, IsImageOutputModel(flagged))
	opted := catwalk.Model{ID: "x-image", Options: catwalk.ModelOptions{ProviderOptions: map[string]any{"image_output": false}}}
	require.False(t, IsImageOutputModel(opted))
}

func TestProviderImageModelPrefersTheChatProvider(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Models: map[SelectedModelType]SelectedModel{SelectedModelTypeLarge: {Provider: "relay", Model: "antigravity/gemini-3-flash"}},
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"another": {Type: catwalk.TypeOpenAICompat, BaseURL: "https://a.example/v1", APIKey: "a", Models: []catwalk.Model{{ID: "gpt-image-1"}}},
			"relay":   {Type: catwalk.TypeOpenAICompat, BaseURL: "https://relay.example/v1/", APIKey: "$RELAY", Models: []catwalk.Model{{ID: "antigravity/gemini-3-flash"}, {ID: "antigravity/gemini-3.1-flash-image"}}},
			"off":     {Type: catwalk.TypeOpenAICompat, BaseURL: "https://off.example/v1", Disable: true, Models: []catwalk.Model{{ID: "z-image"}}},
		}),
	}
	found, ok := cfg.ProviderImageModel()
	require.True(t, ok)
	require.Equal(t, ProviderImageModel{ProviderID: "relay", BaseURL: "https://relay.example/v1", APIKey: "$RELAY", Model: "antigravity/gemini-3.1-flash-image"}, found)

	cfg.Models = nil
	found, ok = cfg.ProviderImageModel()
	require.True(t, ok)
	require.Equal(t, "another", found.ProviderID)

	none := &Config{Providers: csync.NewMapFrom(map[string]ProviderConfig{"relay": {Type: catwalk.TypeOpenAICompat, BaseURL: "https://r/v1", Models: []catwalk.Model{{ID: "chat"}}}})}
	_, ok = none.ProviderImageModel()
	require.False(t, ok)
}
