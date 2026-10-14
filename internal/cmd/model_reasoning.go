package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// modelReasoningCmd displays and manages reasoning configuration for models.
var modelReasoningCmd = &cobra.Command{
	Use:   "reasoning",
	Short: "Display or set reasoning settings for a model",
	Long: `Display or configure reasoning settings for a model using unified reasoning modes.

The reasoning modes are: off, low, medium, high, xhigh, max
These provide fine-grained control over reasoning depth, similar to Claude Code's
effort levels.

Usage:
  # Show reasoning settings for the currently configured large model
  prime model reasoning

  # Show reasoning settings for a specific model
  prime model reasoning [provider/model-id]

  # Set reasoning mode for a model
  prime model reasoning [provider/model-id] --mode high

  # Disable reasoning entirely
  prime model reasoning [provider/model-id] --mode off

The reasoning settings are stored per-model in the provider configuration.

Examples:
  # Check reasoning settings for openai/o3
  prime model reasoning openai/o3

  # Set reasoning mode to high for a model
  prime model reasoning openai/o3 --mode high

  # Set reasoning mode to max for a model
  prime model reasoning openai/o3 --mode max

  # Disable reasoning for an Anthropic model
  prime model reasoning anthropic/claude-3-7-sonnet-20250219 --mode off
`,
	Aliases: []string{"reasoning"},
	Args:    cobra.MaximumNArgs(1),
	RunE:    runModelReasoning,
}

func init() {
	modelCmd.AddCommand(modelReasoningCmd)
	modelReasoningCmd.Flags().String("mode", "", "set reasoning mode (off, low, medium, high, xhigh, max)")
	modelReasoningCmd.Flags().String("effort", "", "deprecated alias for --mode")
	modelReasoningCmd.Flags().Bool("think", false, "deprecated: enable thinking mode (maps to --mode medium)")
	modelReasoningCmd.Flags().Bool("off", false, "disable reasoning entirely (alias for --mode off)")
}

func runModelReasoning(cmd *cobra.Command, args []string) error {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return err
	}

	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")

	cfg, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return err
	}

	// Parse flags - support both new --mode and legacy --effort/--think/--off
	mode, _ := cmd.Flags().GetString("mode")
	effort, _ := cmd.Flags().GetString("effort")
	think, _ := cmd.Flags().GetBool("think")
	thinkFlagSet := cmd.Flags().Changed("think")
	off, _ := cmd.Flags().GetBool("off")
	offFlagSet := cmd.Flags().Changed("off")

	// Normalize legacy flags to new mode
	if off || offFlagSet {
		mode = config.ReasoningModeOff
	}
	if mode == "" && effort != "" {
		mode = config.NormalizeReasoningMode(effort)
	}
	if mode == "" && thinkFlagSet && think {
		mode = config.ReasoningModeMedium
	}
	if mode == "" && thinkFlagSet && !think {
		mode = config.ReasoningModeOff
	}

	// If setting values, we need the model identifier
	if mode != "" || offFlagSet || thinkFlagSet {
		if len(args) == 0 {
			return fmt.Errorf("model identifier is required when setting reasoning parameters (e.g. 'openai/o3')")
		}
		return setModelReasoning(cmd, cfg, args[0], mode)
	}

	// Display mode - no args means current model
	if len(args) > 0 {
		providerID, modelID, err := parseModelIdentifier(args[0])
		if err != nil {
			return err
		}
		return showModelReasoning(cmd, cfg, providerID, modelID)
	}

	// Show current model
	return showCurrentModelReasoning(cmd, cfg)
}

func parseModelIdentifier(identifier string) (string, string, error) {
	parts := strings.SplitN(identifier, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid model identifier: %q. Use format 'provider/model-id'", identifier)
	}
	return parts[0], parts[1], nil
}

func showCurrentModelReasoning(cmd *cobra.Command, cfg *config.ConfigStore) error {
	model := cfg.Config().LargeModel()
	if model == nil {
		return fmt.Errorf("no large model configured")
	}

	providerCfg := cfg.Config().GetProviderForModel(config.SelectedModelTypeLarge)
	if providerCfg == nil {
		return fmt.Errorf("provider for current model not found")
	}

	return printModelReasoning(cmd, providerCfg.ID, model, cfg.Config())
}

func showModelReasoning(cmd *cobra.Command, cfg *config.ConfigStore, providerID, modelID string) error {
	providerCfg, ok := cfg.Config().Providers.Get(providerID)
	if !ok {
		// Check known providers from catwalk
		for _, kp := range cfg.KnownProviders() {
			if string(kp.ID) == providerID {
				for i := range kp.Models {
					if kp.Models[i].ID == modelID {
						return printModelReasoning(cmd, providerID, &kp.Models[i], cfg.Config())
					}
				}
				return fmt.Errorf("model %q not found in provider %q", modelID, providerID)
			}
		}
		return fmt.Errorf("provider %q not found in configuration", providerID)
	}

	for i := range providerCfg.Models {
		if providerCfg.Models[i].ID == modelID {
			return printModelReasoning(cmd, providerCfg.ID, &providerCfg.Models[i], cfg.Config())
		}
	}

	return fmt.Errorf("model %q not found in provider %q", modelID, providerID)
}

func printModelReasoning(cmd *cobra.Command, providerID string, model *catwalk.Model, cfg *config.Config) error {
	// Find user config for this model
	var userCfg *config.SelectedModel
	for _, sm := range cfg.Models {
		if sm.Provider == providerID && sm.Model == model.ID {
			userCfg = &sm
			break
		}
	}
	if userCfg == nil {
		// Default to current model config
		if agentCfg, ok := cfg.Agents[config.AgentGeneral]; ok {
			if sm, ok := cfg.Models[agentCfg.Model]; ok {
				userCfg = &sm
			}
		}
	}

	var currentMode string
	if userCfg != nil {
		currentMode = userCfg.EffectiveReasoningMode()
	}

	cmd.Printf("Provider: %s\n", providerID)
	cmd.Printf("Model:    %s\n", model.ID)
	if model.Name != "" {
		cmd.Printf("Name:     %s\n", model.Name)
	}

	cmd.Printf("Can reason: %t\n", model.CanReason)
	cmd.Printf("Current mode: %s\n", config.FormatReasoningMode(currentMode))
	cmd.Printf("\n")

	cmd.Printf("Available reasoning modes:\n")
	for _, mode := range config.AllReasoningModes() {
		marker := "  "
		if mode == currentMode {
			marker = "* "
		}
		cmd.Printf("  %s%s\n", marker, config.FormatReasoningMode(mode))
	}
	cmd.Printf("  %sOff\n", func() string {
		if currentMode == config.ReasoningModeOff {
			return "* "
		}
		return "  "
	}())

	cmd.Printf("\n")
	cmd.Printf("To set a reasoning mode:\n")
	cmd.Printf("  prime model reasoning %s/%s --mode <level>\n", providerID, model.ID)
	cmd.Printf("\n")
	cmd.Printf("Where <level> is one of: off, low, medium, high, xhigh, max\n")

	return nil
}

func setModelReasoning(cmd *cobra.Command, cfg *config.ConfigStore, identifier, mode string) error {
	providerID, modelID, err := parseModelIdentifier(identifier)
	if err != nil {
		return err
	}

	mode = config.NormalizeReasoningMode(mode)
	if mode == "" {
		// Empty mode means off
		mode = config.ReasoningModeOff
	}

	providerCfg, ok := cfg.Config().Providers.Get(providerID)
	if !ok {
		return fmt.Errorf("provider %q not found in configuration", providerID)
	}

	// Find the model in the provider config
	found := false
	for i := range providerCfg.Models {
		if providerCfg.Models[i].ID == modelID {
			found = true
			// Set the reasoning mode on the model's default config
			providerCfg.Models[i].DefaultReasoningEffort = mode
			if mode == config.ReasoningModeOff {
				providerCfg.Models[i].CanReason = false
			} else {
				providerCfg.Models[i].CanReason = true
			}
			cmd.Printf("Set reasoning mode to %s for %s/%s\n",
				config.FormatReasoningMode(mode), providerID, modelID)
			break
		}
	}

	if !found {
		return fmt.Errorf("model %q not found in provider %q. Add it to your config first.", modelID, providerID)
	}

	// Save the updated provider config using SetConfigField
	modelsJSON, err := json.Marshal(providerCfg.Models)
	if err != nil {
		return fmt.Errorf("failed to marshal model settings: %w", err)
	}

	configPath := fmt.Sprintf("providers.%s.models", providerID)
	if err := cfg.SetConfigField(config.ScopeGlobal, configPath, parseJSONArray(string(modelsJSON))); err != nil {
		return fmt.Errorf("failed to save provider configuration: %w", err)
	}

	// Also update the user's SelectedModel preferences
	agentCfg := cfg.Config().Agents[config.AgentGeneral]
	currentModel := cfg.Config().Models[agentCfg.Model]
	currentModel.ReasoningMode = mode
	if mode == config.ReasoningModeOff {
		currentModel.ReasoningEffort = ""
		currentModel.Think = false
	}
	if err := cfg.SetConfigField(config.ScopeGlobal, fmt.Sprintf("models.%s", agentCfg.Model), currentModel); err != nil {
		cmd.Printf("Warning: could not update model preferences: %v\n", err)
	}

	return nil
}

// parseJSONArray converts a JSON array string to a format that can be used with SetConfigField
func parseJSONArray(s string) interface{} {
	var arr []catwalk.Model
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return arr
	}
	return s
}
