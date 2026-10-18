package anthropic

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestLacksAdaptiveThinking(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"claude-3-7-sonnet-20250219":                   true,
		"claude-haiku-4-5-20251001":                    true,
		"claude-opus-4-20250514":                       true,
		"claude-opus-4-1-20250805":                     true,
		"claude-opus-4-5-20251101":                     true,
		"claude-sonnet-4-5-20250929":                   true,
		"anthropic.claude-sonnet-4-5-20250929-v1:0":    true,
		"claude-sonnet-4-6":                            false,
		"claude-opus-4-7":                              false,
		"claude-opus-5":                                false,
		"claude-fable-5-1":                             false,
		"us.anthropic.claude-opus-4-6-20260101-v1:0":   false,
		"custom-model":                                 false,
	}
	for model, want := range tests {
		require.Equal(t, want, lacksAdaptiveThinking(model), model)
	}
}

func TestEffortWithoutAdaptiveThinkingUsesBudget(t *testing.T) {
	t.Parallel()

	effort := EffortHigh
	temperature := 0.5
	model := languageModel{modelID: "claude-opus-4-5-20251101"}
	params, _, warnings, _, err := model.prepareParams(fantasy.Call{
		Prompt:      fantasy.Prompt{fantasy.NewUserMessage("hi")},
		Temperature: &temperature,
		ProviderOptions: fantasy.ProviderOptions{Name: &ProviderOptions{
			Effort:   &effort,
			Thinking: &ThinkingProviderOption{BudgetTokens: 4096},
		}},
	})
	require.NoError(t, err)
	require.Nil(t, params.Thinking.OfAdaptive)
	require.NotNil(t, params.Thinking.OfEnabled)
	require.Equal(t, int64(4096), params.Thinking.OfEnabled.BudgetTokens)
	require.False(t, params.Temperature.Valid())
	require.NotEmpty(t, warnings)
}

func TestEffortWithAdaptiveThinking(t *testing.T) {
	t.Parallel()

	effort := EffortHigh
	model := languageModel{modelID: "claude-opus-4-7"}
	params, _, _, _, err := model.prepareParams(fantasy.Call{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage("hi")},
		ProviderOptions: fantasy.ProviderOptions{Name: &ProviderOptions{
			Effort:   &effort,
			Thinking: &ThinkingProviderOption{BudgetTokens: 4096},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, params.Thinking.OfAdaptive)
	require.Nil(t, params.Thinking.OfEnabled)
}
