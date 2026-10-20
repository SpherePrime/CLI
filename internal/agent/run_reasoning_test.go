package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRunReasoningModesAreIndependent(t *testing.T) {
	base := Model{CatwalkCfg: catwalk.Model{ID: "gpt-5", CanReason: true, ReasoningLevels: []string{"none", "low", "medium", "high"}}, ModelCfg: config.SelectedModel{ReasoningMode: "low", Think: true, ReasoningEffort: "low"}}
	high := base
	high.ModelCfg = applyRunReasoningMode(WithReasoningMode(context.Background(), "high"), base.ModelCfg)
	off := base
	off.ModelCfg = applyRunReasoningMode(WithReasoningMode(context.Background(), "off"), base.ModelCfg)
	highOptions := getProviderOptions(high, config.ProviderConfig{Type: openai.Name})[openai.Name].(*openai.ResponsesProviderOptions)
	offOptions := getProviderOptions(off, config.ProviderConfig{Type: openai.Name})[openai.Name].(*openai.ResponsesProviderOptions)
	require.Equal(t, "high", string(*highOptions.ReasoningEffort))
	require.Equal(t, "none", string(*offOptions.ReasoningEffort))
	require.Equal(t, "low", base.ModelCfg.ReasoningMode)
	require.Equal(t, base.ModelCfg, applyRunReasoningMode(context.Background(), base.ModelCfg))
	defaults := applyRunReasoningMode(WithReasoningMode(context.Background(), "default"), base.ModelCfg)
	require.Empty(t, defaults.ReasoningMode)
	require.Empty(t, defaults.ReasoningEffort)
	require.False(t, defaults.Think)
}
