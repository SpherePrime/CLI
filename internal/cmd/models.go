package cmd

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2/tree"
	"github.com/SpherePrime/CLI/vendordeps/mattn/go-isatty"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from known providers",
	Long:  `List all available models from known providers. Shows provider name and model IDs. Unconfigured providers are marked with (not configured).`,
	Example: `# List all available models
prime models

# Search models
prime models gpt5

# Show which models support reasoning
prime models --reasoning`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		term := strings.ToLower(strings.Join(args, " "))
		showReasoning, _ := cmd.Flags().GetBool("reasoning")

		type modelEntry struct {
			id         string
			reasons    bool
			levels     []string
		}
		type providerEntry struct {
			name       string
			models     []modelEntry
			configured bool
		}

		entries := make(map[string]*providerEntry)

		// Add configured providers first.
		for providerID, provider := range cfg.Config().Providers.Seq2() {
			if provider.Disable {
				continue
			}
			entry := &providerEntry{
				name:       provider.Name,
				configured: true,
			}

			// The OpenAI provider holds exactly one credential. Signed
			// in with ChatGPT, only the models the subscription grants are
			// usable; an API key lists the regular catalog.
			var models []catwalk.Model
			if providerID == string(catwalk.InferenceProviderOpenAI) && provider.OAuthToken != nil {
				models = provider.ChatGPTModels
			} else {
				models = provider.Models
			}

			for _, model := range models {
				if term != "" {
					matched := false
					for _, s := range []string{provider.ID, provider.Name, model.ID, model.Name} {
						if strings.Contains(strings.ToLower(s), term) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				entry.models = append(entry.models, modelEntry{
					id:      model.ID,
					reasons: model.CanReason,
					levels:  model.ReasoningLevels,
				})
			}
			if len(entry.models) > 0 {
				slices.SortFunc(entry.models, func(a, b modelEntry) int {
					return strings.Compare(a.id, b.id)
				})
				entries[providerID] = entry
			}
		}

		// Add known but unconfigured providers from catwalk.
		for _, kp := range cfg.KnownProviders() {
			providerID := string(kp.ID)
			if _, exists := entries[providerID]; exists {
				continue
			}
			entry := &providerEntry{
				name:       kp.Name,
				configured: false,
			}
			for _, model := range kp.Models {
				if term != "" {
					matched := false
					for _, s := range []string{providerID, kp.Name, model.ID, model.Name} {
						if strings.Contains(strings.ToLower(s), term) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				entry.models = append(entry.models, modelEntry{
					id:      model.ID,
					reasons: model.CanReason,
					levels:  model.ReasoningLevels,
				})
			}
			if len(entry.models) > 0 {
				slices.SortFunc(entry.models, func(a, b modelEntry) int {
					return strings.Compare(a.id, b.id)
				})
				entries[providerID] = entry
			}
		}

		var providerIDs []string
		for id := range entries {
			providerIDs = append(providerIDs, id)
		}
		sort.Strings(providerIDs)

		if len(providerIDs) == 0 && len(args) == 0 {
			return fmt.Errorf("no providers found")
		}
		if len(providerIDs) == 0 {
			return fmt.Errorf("no providers found matching %q", term)
		}

		if !isatty.IsTerminal(os.Stdout.Fd()) {
			for _, providerID := range providerIDs {
				entry := entries[providerID]
				for _, model := range entry.models {
					if showReasoning {
						reasoning := "false"
						if model.reasons {
							reasoning = "true"
						}
						fmt.Printf("%s/%s\t%s\n", providerID, model.id, reasoning)
					} else {
						fmt.Println(providerID + "/" + model.id)
					}
				}
			}
			return nil
		}

		t := tree.New()
		for _, providerID := range providerIDs {
			entry := entries[providerID]
			label := providerID
			if !entry.configured {
				label += " (not configured)"
			}
			providerNode := tree.Root(label)
			for _, model := range entry.models {
				modelLabel := model.id
				if showReasoning && model.reasons {
					modelLabel += " [reasoning"
					if len(model.levels) > 0 {
						modelLabel += ": " + strings.Join(model.levels, ",")
					}
					modelLabel += "]"
				} else if showReasoning && !model.reasons {
					modelLabel += " [no reasoning]"
				}
				providerNode.Child(modelLabel)
			}
			t.Child(providerNode)
		}

		cmd.Println(t)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.Flags().BoolP("reasoning", "r", false, "Show reasoning support for each model")
}
