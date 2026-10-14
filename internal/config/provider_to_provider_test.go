package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProviderConfig_ToProvider(t *testing.T) {
	t.Parallel()

	t.Run("copies all model fields including TokenCounting and Options", func(t *testing.T) {
		t.Parallel()

		cfg := &ProviderConfig{
			Name:   "Test Provider",
			ID:     "test",
			Models: []catwalk.Model{
				{
					ID:                     "model-a",
					Name:                   "Model A",
					CostPer1MIn:            1.0,
					CostPer1MOut:           2.0,
					CostPer1MInCached:      0.5,
					CostPer1MOutCached:     0.8,
					ContextWindow:          128000,
					DefaultMaxTokens:       4096,
					CanReason:              true,
					ReasoningLevels:        []string{"low", "medium", "high"},
					DefaultReasoningEffort: "medium",
					SupportsImages:         true,
					TokenCounting:          "native",
					Options: catwalk.ModelOptions{
						Temperature: float64Ptr(0.7),
						TopP:        float64Ptr(0.9),
					},
				},
			},
		}

		provider := cfg.ToProvider()

		require.Equal(t, "Test Provider", provider.Name)
		require.Equal(t, catwalk.InferenceProvider("test"), provider.ID)
		require.Len(t, provider.Models, 1)

		m := provider.Models[0]
		require.Equal(t, "model-a", m.ID)
		require.Equal(t, "Model A", m.Name)
		require.Equal(t, 1.0, m.CostPer1MIn)
		require.Equal(t, 2.0, m.CostPer1MOut)
		require.Equal(t, 0.5, m.CostPer1MInCached)
		require.Equal(t, 0.8, m.CostPer1MOutCached)
		require.Equal(t, int64(128000), m.ContextWindow)
		require.Equal(t, int64(4096), m.DefaultMaxTokens)
		require.True(t, m.CanReason)
		require.Equal(t, []string{"low", "medium", "high"}, m.ReasoningLevels)
		require.Equal(t, "medium", m.DefaultReasoningEffort)
		require.True(t, m.SupportsImages)
		require.Equal(t, "native", m.TokenCounting)
		require.NotNil(t, m.Options.Temperature)
		require.Equal(t, 0.7, *m.Options.Temperature)
		require.NotNil(t, m.Options.TopP)
		require.Equal(t, 0.9, *m.Options.TopP)
	})

	t.Run("handles empty models", func(t *testing.T) {
		t.Parallel()

		cfg := &ProviderConfig{
			Name: "Empty Provider",
			ID:   "empty",
		}

		provider := cfg.ToProvider()
		require.Equal(t, "Empty Provider", provider.Name)
		require.Equal(t, catwalk.InferenceProvider("empty"), provider.ID)
		require.Empty(t, provider.Models)
	})

	t.Run("handles nil Options", func(t *testing.T) {
		t.Parallel()

		cfg := &ProviderConfig{
			ID: "test",
			Models: []catwalk.Model{
				{ID: "m1", Name: "M1"},
			},
		}

		provider := cfg.ToProvider()
		require.Len(t, provider.Models, 1)
		require.Equal(t, catwalk.ModelOptions{}, provider.Models[0].Options)
	})
}

func float64Ptr(v float64) *float64 {
	return &v
}
