package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/discover"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// contextWindowOverride is the context window (in tokens) that the provider
// add command assigns to every discovered model.
const contextWindowOverride = 270_000

// providerAddDiscoveryTimeout bounds the network call to the provider's
// /models endpoint.
const providerAddDiscoveryTimeout = 3 * time.Second

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage LLM providers",
}

var providerAddCmd = &cobra.Command{
	Use:   "add [id]",
	Short: "Add a custom provider",
	Long: `Add a custom LLM provider by prompting for its ID, base URL, and API key.

When run interactively, models are fetched from the provider's /models
endpoint and every model gets a 270k token context window. The result is
written to the global config in the user config directory
(~/.config/prime, one JSON file per section) unless --workspace is set.`,
	Example: `# Add a provider interactively
prime provider add

# Add with all details supplied upfront
prime provider add my-llm --base-url https://host/v1 --api-key sk-...`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" && len(args) > 0 {
			id = args[0]
		}
		name, _ := cmd.Flags().GetString("name")
		baseURL, _ := cmd.Flags().GetString("base-url")
		apiKey, _ := cmd.Flags().GetString("api-key")
		if apiKey == "" {
			apiKey = os.Getenv("PRIME_PROVIDER_API_KEY")
		}
		providerType, _ := cmd.Flags().GetString("type")
		workspace, _ := cmd.Flags().GetBool("workspace")
		discoverModels, _ := cmd.Flags().GetBool("discover")
		contextWindow, _ := cmd.Flags().GetInt("context-window")
		if contextWindow <= 0 {
			contextWindow = contextWindowOverride
		}
		if providerType == "" {
			providerType = string(catwalk.TypeOpenAICompat)
		}

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

		interactive := term.IsTerminal(os.Stdin.Fd())

		if id == "" {
			if !interactive {
				return errors.New("provider ID is required (--id flag or argument)")
			}
			if id, err = promptLine(cmd, "Provider ID"); err != nil {
				return err
			}
		}
		id = strings.TrimSpace(id)
		if id == "" {
			return errors.New("provider ID must not be empty")
		}
		if strings.ContainsAny(id, "/ ") {
			return fmt.Errorf("invalid provider ID %q: must not contain '/' or spaces", id)
		}

		if baseURL == "" {
			if !interactive {
				return errors.New("base URL is required (--base-url)")
			}
			if baseURL, err = promptLine(cmd, "Base URL"); err != nil {
				return err
			}
		}
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			return errors.New("base URL must not be empty")
		}
		if u, err := url.Parse(baseURL); err != nil || (u.Scheme == "" && u.Host == "") {
			return fmt.Errorf("invalid base URL %q: must be a valid absolute URL", baseURL)
		}

		if apiKey == "" {
			if !interactive {
				return errors.New("API key is required (--api-key or PRIME_PROVIDER_API_KEY)")
			}
			if apiKey, err = promptPassword(cmd, "API key"); err != nil {
				return err
			}
		}
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			return errors.New("API key must not be empty")
		}

		var models []catwalk.Model
		if discoverModels {
			probeCtx, cancel := context.WithTimeout(cmd.Context(), providerAddDiscoveryTimeout)
			defer cancel()
			discovered, err := discover.DiscoverModels(probeCtx, discover.Config{
				ID:      id,
				BaseURL: baseURL,
				APIKey:  apiKey,
			}, cfg.Resolver())
			if err != nil {
				return fmt.Errorf("failed to discover models from %s: %w", baseURL, err)
			}
			models = discovered
		}

		// Every model gets the configured context window.
		for i := range models {
			models[i].ContextWindow = int64(contextWindow)
		}

		if name == "" {
			name = id
		}

		scope := config.ScopeGlobal
		if workspace {
			scope = config.ScopeWorkspace
		}
		if err := cfg.SetConfigFields(scope, map[string]any{
			"providers." + id + ".name":            name,
			"providers." + id + ".base_url":        baseURL,
			"providers." + id + ".api_key":         apiKey,
			"providers." + id + ".type":            providerType,
			"providers." + id + ".discover_models": true,
			"providers." + id + ".models":          models,
		}); err != nil {
			return fmt.Errorf("failed to save provider: %w", err)
		}

		printProviderAddSuccess(id, name, baseURL, len(models), contextWindow, workspace)
		return nil
	},
}

// promptLine prints a label and reads one trimmed line from stdin.
func promptLine(cmd *cobra.Command, label string) (string, error) {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: ", label); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptPassword reads a hidden API key from stdin using x/term.
func promptPassword(cmd *cobra.Command, label string) (string, error) {
	if _, err := fmt.Fprint(cmd.OutOrStdout(), label+": "); err != nil {
		return "", err
	}
	raw, err := term.ReadPassword(os.Stdin.Fd())
	if err != nil {
		return "", err
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return strings.TrimSpace(string(raw)), nil
}

func printProviderAddSuccess(id, name, baseURL string, modelCount, contextWindow int, workspace bool) {
	header := lipgloss.NewStyle().
		Foreground(colortone.Butter).
		Background(colortone.Guac).
		Bold(true).
		Padding(0, 1).
		Margin(1).
		MarginLeft(2).
		SetString("SUCCESS")
	scope := "global"
	if workspace {
		scope = "workspace"
	}
	detail := lipgloss.NewStyle().
		MarginLeft(2).
		SetString(fmt.Sprintf(
			"Provider %q (%s) added: %d models with a %d token context window, base URL %s, saved to %s scope.",
			id, name, modelCount, contextWindow, baseURL, scope,
		))
	fmt.Printf("%s\n%s\n\n", header.Render(), detail.Render())
}

func init() {
	providerAddCmd.Flags().String("id", "", "Provider ID (key under providers; positional argument works too)")
	providerAddCmd.Flags().String("name", "", "Display name for the provider (defaults to the ID)")
	providerAddCmd.Flags().String("base-url", "", "Base URL of the provider's API")
	providerAddCmd.Flags().String("api-key", "", "API key for the provider (PRIME_PROVIDER_API_KEY works too)")
	providerAddCmd.Flags().String("type", string(catwalk.TypeOpenAICompat), "Provider API type (openai, anthropic, google, openai-compat, ...)")
	providerAddCmd.Flags().Bool("workspace", false, "Save to the workspace config instead of the global config")
	providerAddCmd.Flags().Bool("discover", true, "Fetch the model list from the provider's /models endpoint")
	providerAddCmd.Flags().Int("context-window", contextWindowOverride, "Context window in tokens assigned to every model")

	providerCmd.AddCommand(providerAddCmd)
}
