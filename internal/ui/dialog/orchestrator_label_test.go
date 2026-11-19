package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestOrchestratorModelLabelShowsReasoningLevel(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("anthropic", config.ProviderConfig{
		ID: "anthropic",
		Models: []catwalk.Model{
			{
				ID:              "claude-sonnet-4",
				Name:            "Claude Sonnet 4",
				CanReason:       true,
				ReasoningLevels: []string{"low", "medium", "high"},
			},
			{
				ID:        "claude-haiku",
				Name:      "Claude Haiku",
				CanReason: false,
			},
		},
	})
	cfg := &config.Config{Providers: providers}

	require.Equal(t,
		"Claude Sonnet 4 · high",
		orchestratorModelLabel(cfg, "planner", config.SelectedModel{Provider: "anthropic", Model: "claude-sonnet-4"}, false),
	)
	require.Equal(t,
		"Claude Sonnet 4 (auto) · high",
		orchestratorModelLabel(cfg, "planner", config.SelectedModel{Provider: "anthropic", Model: "claude-sonnet-4"}, true),
	)
	require.Equal(t,
		"Claude Sonnet 4 (auto) · medium",
		orchestratorModelLabel(cfg, "executor", config.SelectedModel{Provider: "anthropic", Model: "claude-sonnet-4"}, true),
	)
	require.Equal(t,
		"Claude Sonnet 4 · low",
		orchestratorModelLabel(cfg, "executor", config.SelectedModel{Provider: "anthropic", Model: "claude-sonnet-4", ReasoningMode: config.ReasoningModeLow}, false),
	)
	require.Equal(t,
		"Claude Haiku",
		orchestratorModelLabel(cfg, "executor", config.SelectedModel{Provider: "anthropic", Model: "claude-haiku"}, false),
	)
}
