package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/instructions"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestInstructionRoutingRespectsToolAvailabilityAndAgentMode(t *testing.T) {
	env := testEnv(t)
	cfg := writeSmartToolsConfig(t, env.workingDir)
	for _, builder := range []func(...prompt.Option) (*prompt.Prompt, error){coderPrompt, codePrompt, taskPrompt, planPrompt} {
		instructionPrompt, err := builder(prompt.WithWorkingDir(env.workingDir))
		require.NoError(t, err)
		body, err := instructionPrompt.Build(context.Background(), "mock", "mock-model", cfg)
		require.NoError(t, err)
		require.Contains(t, body, "<task_instructions>", instructionPrompt.Name())
		require.Contains(t, body, "read_instruction", instructionPrompt.Name())
	}
	cfg.Config().Options.DisabledTools = []string{"read_instruction"}
	cfg.SetupAgents()
	for _, builder := range []func(...prompt.Option) (*prompt.Prompt, error){coderPrompt, codePrompt, taskPrompt, planPrompt} {
		instructionPrompt, err := builder(prompt.WithWorkingDir(env.workingDir))
		require.NoError(t, err)
		body, err := instructionPrompt.Build(context.Background(), "mock", "mock-model", cfg)
		require.NoError(t, err)
		require.NotContains(t, body, "<task_instructions>", instructionPrompt.Name())
	}
	require.NotContains(t, cfg.Config().Agents[config.AgentPlan].AllowedTools, "bash")
}

func TestInitialAgentPromptsDoNotLoadInstructionBodies(t *testing.T) {
	env := testEnv(t)
	cfg := writeSmartToolsConfig(t, env.workingDir)
	for _, builder := range []func(...prompt.Option) (*prompt.Prompt, error){coderPrompt, codePrompt, taskPrompt, planPrompt} {
		instructionPrompt, err := builder(prompt.WithWorkingDir(env.workingDir))
		require.NoError(t, err)
		body, err := instructionPrompt.Build(context.Background(), "mock", "mock-model", cfg)
		require.NoError(t, err)
		for _, entry := range instructions.Catalog() {
			module, err := instructions.Read(entry.ID)
			require.NoError(t, err)
			require.NotContains(t, body, module, entry.ID)
		}
	}
}
