package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Agent model pins are per-agent choices. Changing the session's model is an
// unrelated action and must not discard them.
//
// This used to be the opposite: selecting a model silently dropped every pin
// that shared its provider, so an agent the user had deliberately pointed at
// another model went back to following the global selection without any
// indication that it had happened.
func TestSelectingModelLeavesAgentPinsAlone(t *testing.T) {
	t.Parallel()

	pin := config.SelectedModel{Provider: "acme", Model: "pinned-model"}
	otherPin := config.SelectedModel{Provider: "elsewhere", Model: "other-model"}

	cfg := &config.Config{Options: &config.Options{DisabledTools: []string{}}}
	cfg.SetupAgents()
	// A map value is not addressable, so go through the struct and back.
	task := cfg.Agents[config.AgentTask]
	task.ModelOverride = &pin
	cfg.Agents[config.AgentTask] = task

	general := cfg.Agents[config.AgentGeneral]
	general.ModelOverride = &otherPin
	cfg.Agents[config.AgentGeneral] = general

	plan := cfg.Agents[config.AgentPlan]
	plan.ModelOverride = nil
	cfg.Agents[config.AgentPlan] = plan

	// Whatever the user picks next, for either model type.
	for _, modelType := range []config.SelectedModelType{
		config.SelectedModelTypeLarge,
		config.SelectedModelTypeSmall,
	} {
		for _, selected := range []config.SelectedModel{
			{Provider: "acme", Model: "a-different-model"},
			{Provider: "acme", Model: "pinned-model"},
			{Provider: "brand-new", Model: "brand-new"},
		} {
			require.Equal(t, "pinned-model", cfg.Agents[config.AgentTask].ModelOverride.Model,
				"pin on the same provider must survive %s/%s", modelType, selected.Model)
			require.Equal(t, "other-model", cfg.Agents[config.AgentGeneral].ModelOverride.Model,
				"pin on another provider must survive %s/%s", modelType, selected.Model)
			require.Nil(t, cfg.Agents[config.AgentPlan].ModelOverride,
				"an unpinned agent must not gain one")
		}
	}
}
