package cmd

import (
	"fmt"
	"log/slog"

	"github.com/dwertyfa288/CLI/vendordeps/lipgloss/v2"
	"github.com/dwertyfa288/CLI/internal/config"
	"github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/x/exp/colortone"
	"github.com/dwertyfa288/CLI/vendordeps/spf13/cobra"
)

var updateProvidersSource string

var updateProvidersCmd = &cobra.Command{
	Use:   "update-providers [path-or-url]",
	Short: "Update providers",
	Long:  `Update provider information from a specified local path or remote URL.`,
	Example: `
# Update Catwalk providers remotely (default)
prime update-providers

# Update Catwalk providers from a custom URL
prime update-providers https://example.com/providers.json

# Update Catwalk providers from a local file
prime update-providers /path/to/local-providers.json

# Update Catwalk providers from embedded version
prime update-providers embedded

# Update Hyper provider information
prime update-providers --source=hyper

# Update Hyper from a custom URL
prime update-providers --source=hyper https://hyper.example.com
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// NOTE(@dwertyfa288): We want to skip logging output do stdout here.
		slog.SetDefault(slog.New(slog.DiscardHandler))

		var pathOrURL string
		if len(args) > 0 {
			pathOrURL = args[0]
		}

		var err error
		switch updateProvidersSource {
		case "catwalk":
			err = config.UpdateProviders(pathOrURL)
		case "hyper":
			err = config.UpdateHyper(pathOrURL)
		default:
			return fmt.Errorf("invalid source %q, must be 'catwalk' or 'hyper'", updateProvidersSource)
		}

		if err != nil {
			return err
		}

		// NOTE(@dwertyfa288): This style is more-or-less copied from Fang's
		// error message, adapted for success.
		headerStyle := lipgloss.NewStyle().
			Foreground(colortone.Butter).
			Background(colortone.Guac).
			Bold(true).
			Padding(0, 1).
			Margin(1).
			MarginLeft(2).
			SetString("SUCCESS")
		textStyle := lipgloss.NewStyle().
			MarginLeft(2).
			SetString(fmt.Sprintf("%s provider updated successfully.", updateProvidersSource))

		fmt.Printf("%s\n%s\n\n", headerStyle.Render(), textStyle.Render())
		return nil
	},
}

func init() {
	updateProvidersCmd.Flags().StringVar(&updateProvidersSource, "source", "catwalk", "Provider source to update (catwalk or hyper)")
}
