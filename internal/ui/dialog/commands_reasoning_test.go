package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCommandsOffersDeclaredReasoningLevels(t *testing.T) {
	c := newTestCommands(t)
	cfg := c.com.Config()
	cfg.SetupAgents()
	slot := cfg.Agents[config.AgentGeneral].Model
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{slot: {Model: "reasoning-model", Provider: "test", ReasoningEffort: "high"}}
	cfg.Providers.Set("test", config.ProviderConfig{ID: "test", Models: []catwalk.Model{{ID: "reasoning-model", CanReason: true, ReasoningLevels: []string{"low", "high"}}}})
	c.com.Workspace = &defaultModelWorkspace{cfg: *cfg}
	for _, item := range c.defaultCommands() {
		if item.ID() == "select_reasoning_effort" {
			require.Contains(t, item.title, "High")
			require.Equal(t, ActionOpenDialog{DialogID: ReasoningID}, item.action)
			return
		}
	}
	t.Fatal("reasoning selection must be available for declared levels")
}
