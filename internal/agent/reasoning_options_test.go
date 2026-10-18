package agent

import (
	"fmt"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/google"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func anthropicOptions(t *testing.T, model Model) *anthropic.ProviderOptions {
	t.Helper()
	opts := getProviderOptions(model, config.ProviderConfig{ID: "test", Type: anthropic.Name})
	parsed, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
	require.True(t, ok)
	return parsed
}

func TestExplicitReasoningOffDisablesCatalogDefault(t *testing.T) {
	t.Parallel()

	parsed := anthropicOptions(t, Model{
		CatwalkCfg: catwalk.Model{ID: "claude-opus-4-7", CanReason: true, ReasoningLevels: []string{"low", "high"}, DefaultReasoningEffort: "high"},
		ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeOff},
	})

	require.Nil(t, parsed.Effort)
	require.Nil(t, parsed.Thinking)
}

func TestAnthropicModelWithoutEffortLevelsUsesThinkingBudget(t *testing.T) {
	t.Parallel()

	parsed := anthropicOptions(t, Model{
		CatwalkCfg: catwalk.Model{ID: "claude-sonnet-4-5-20250929", CanReason: true, DefaultMaxTokens: 64000},
		ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeHigh},
	})

	require.Nil(t, parsed.Effort)
	require.NotNil(t, parsed.Thinking)
	require.Equal(t, anthropicThinkingBudgets[config.ReasoningModeHigh], parsed.Thinking.BudgetTokens)
}

func TestAnthropicEffortIsClampedToModelLevels(t *testing.T) {
	t.Parallel()

	parsed := anthropicOptions(t, Model{
		CatwalkCfg: catwalk.Model{ID: "claude-sonnet-4-6", CanReason: true, ReasoningLevels: []string{"low", "medium", "high", "max"}},
		ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeXHigh},
	})

	require.NotNil(t, parsed.Effort)
	require.Equal(t, anthropic.Effort("high"), *parsed.Effort)
}

func TestAnthropicThinkingBudgetFitsMaxTokens(t *testing.T) {
	t.Parallel()

	model := Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 8192}}
	require.Equal(t, int64(8192-anthropicMinThinkingBudget), anthropicThinkingBudget(model, config.ReasoningModeMax))

	model.ModelCfg.MaxTokens = 1500
	require.Equal(t, int64(anthropicMinThinkingBudget), anthropicThinkingBudget(model, config.ReasoningModeMax))
}

func TestOpenAIReasoningEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode   string
		levels []string
		want   string
	}{
		{mode: config.ReasoningModeXHigh, levels: []string{"low", "medium", "high", "xhigh"}, want: "xhigh"},
		{mode: config.ReasoningModeMax, levels: []string{"low", "medium", "high", "xhigh"}, want: "xhigh"},
		{mode: config.ReasoningModeOff, levels: []string{"none", "low"}, want: "none"},
		{mode: config.ReasoningModeOff, levels: []string{"minimal", "low"}, want: "minimal"},
		{mode: config.ReasoningModeMax, want: "high"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s/%v", tc.mode, tc.levels), func(t *testing.T) {
			t.Parallel()
			model := Model{
				CatwalkCfg: catwalk.Model{ID: "gpt-5.4", CanReason: true, ReasoningLevels: tc.levels},
				ModelCfg:   config.SelectedModel{ReasoningMode: tc.mode},
			}
			opts := getProviderOptions(model, config.ProviderConfig{ID: "openai", Type: openai.Name})
			parsed, ok := opts[openai.Name].(*openai.ResponsesProviderOptions)
			require.True(t, ok)
			require.NotNil(t, parsed.ReasoningEffort)
			require.Equal(t, tc.want, string(*parsed.ReasoningEffort))
		})
	}
}

func googleOptions(t *testing.T, model Model) *google.ProviderOptions {
	t.Helper()
	opts := getProviderOptions(model, config.ProviderConfig{ID: "gemini", Type: google.Name})
	parsed, ok := opts[google.Name].(*google.ProviderOptions)
	require.True(t, ok)
	return parsed
}

func TestGoogleThinkingConfig(t *testing.T) {
	t.Parallel()

	t.Run("gemini 3 clamps level", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "gemini-3-pro-preview", CanReason: true, ReasoningLevels: []string{"low", "high"}},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeMax},
		})
		require.NotNil(t, parsed.ThinkingConfig)
		require.NotNil(t, parsed.ThinkingConfig.ThinkingLevel)
		require.Equal(t, "high", *parsed.ThinkingConfig.ThinkingLevel)
		require.Nil(t, parsed.ThinkingConfig.ThinkingBudget)
	})

	t.Run("gemini 3 off uses minimal level", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "gemini-3-flash-preview", CanReason: true, ReasoningLevels: []string{"minimal", "low", "medium", "high"}},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeOff},
		})
		require.NotNil(t, parsed.ThinkingConfig.ThinkingLevel)
		require.Equal(t, "minimal", *parsed.ThinkingConfig.ThinkingLevel)
	})

	t.Run("gemini 2.5 flash maps mode to budget", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "gemini-2.5-flash", CanReason: true},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeMax},
		})
		require.NotNil(t, parsed.ThinkingConfig.ThinkingBudget)
		require.Equal(t, int64(geminiFlashMaxBudget), *parsed.ThinkingConfig.ThinkingBudget)
	})

	t.Run("gemini 2.5 flash off disables thinking", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "gemini-2.5-flash", CanReason: true},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeOff},
		})
		require.NotNil(t, parsed.ThinkingConfig.ThinkingBudget)
		require.Equal(t, int64(0), *parsed.ThinkingConfig.ThinkingBudget)
	})

	t.Run("non reasoning model gets no thinking config", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{CatwalkCfg: catwalk.Model{ID: "gemini-2.0-flash"}})
		require.Nil(t, parsed.ThinkingConfig)
	})
}

func TestCustomProviderReasoningOptions(t *testing.T) {
	t.Parallel()

	t.Run("unknown anthropic compatible model uses budget within default max tokens", func(t *testing.T) {
		t.Parallel()
		parsed := anthropicOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "kimi-k2-custom"},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeMax},
		})
		require.Nil(t, parsed.Effort)
		require.NotNil(t, parsed.Thinking)
		require.Equal(t, int64(anthropicDefaultMaxTokens-anthropicMinThinkingBudget), parsed.Thinking.BudgetTokens)
	})

	t.Run("custom claude alias with adaptive support uses effort", func(t *testing.T) {
		t.Parallel()
		parsed := anthropicOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "proxy/claude-opus-4-7"},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeHigh},
		})
		require.NotNil(t, parsed.Effort)
		require.Equal(t, anthropic.Effort("high"), *parsed.Effort)
	})

	t.Run("custom gemini model clamps level to standard range", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{
			CatwalkCfg: catwalk.Model{ID: "my-gemini-3"},
			ModelCfg:   config.SelectedModel{ReasoningMode: config.ReasoningModeMax},
		})
		require.NotNil(t, parsed.ThinkingConfig.ThinkingLevel)
		require.Equal(t, "high", *parsed.ThinkingConfig.ThinkingLevel)
	})

	t.Run("custom gemini model without explicit mode keeps provider default", func(t *testing.T) {
		t.Parallel()
		parsed := googleOptions(t, Model{CatwalkCfg: catwalk.Model{ID: "my-gemini-3"}})
		require.Nil(t, parsed.ThinkingConfig)
	})
}
