package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestInstructionToolsAreAvailableInAllAgentModes(t *testing.T) {
	for _, smart := range []bool{false, true} {
		coord := newGateTestCoordinator(t, true)
		coord.cfg.Config().Options.SmartTools = smart
		for _, mode := range []string{config.AgentGeneral, config.AgentCode, config.AgentTask, config.AgentPlan} {
			built, err := coord.buildTools(context.Background(), coord.cfg.Config().Agents[mode], false)
			require.NoError(t, err)
			names := toolNamesOf(built)
			require.Contains(t, names, "search_instructions", mode)
			require.Contains(t, names, "read_instruction", mode)
			if mode == config.AgentTask || mode == config.AgentPlan {
				require.NotContains(t, names, "write")
				require.NotContains(t, names, "bash")
			}
		}
	}
}

func TestInstructionToolsHonorDisabledTools(t *testing.T) {
	coord := newGateTestCoordinator(t, true)
	coord.cfg.Config().Options.DisabledTools = []string{"search_instructions", "read_instruction"}
	coord.cfg.SetupAgents()
	built, err := coord.buildTools(context.Background(), coord.cfg.Config().Agents[config.AgentGeneral], false)
	require.NoError(t, err)
	require.NotContains(t, toolNamesOf(built), "search_instructions")
	require.NotContains(t, toolNamesOf(built), "read_instruction")
}

func TestSmartSearchDoesNotAdvertiseDisabledInstructionTools(t *testing.T) {
	coord := newGateTestCoordinator(t, true)
	coord.cfg.Config().Options.SmartTools = true
	coord.cfg.Config().Options.DisabledTools = []string{"read_instruction"}
	coord.cfg.SetupAgents()
	built, err := coord.buildTools(context.Background(), coord.cfg.Config().Agents[config.AgentGeneral], false)
	require.NoError(t, err)
	search := builtToolByName(t, built, "search_tools")
	require.NotNil(t, search)
	response, err := search.Run(context.Background(), fantasy.ToolCall{Input: `{"query":"read_instruction"}`})
	require.NoError(t, err)
	require.NotContains(t, response.Content, "Call read_instruction")
}
