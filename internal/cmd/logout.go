package cmd

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/client"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/logout"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// providerDisplayNames maps OAuth-capable provider IDs to display names.
// Keep this list in sync with the switch in RunE and the login command.
var providerDisplayNames = map[string]string{
	"hyper":   "Hyper",
	"copilot": "GitHub Copilot",
	"openai":  "ChatGPT",
}

var logoutCmd = &cobra.Command{
	Aliases: []string{"signout"},
	Use:     "logout [platform]",
	Short:   "Logout Prime from a platform",
	Long: `Logout from your Prime account or a specified model platform, removing stored credentials.
If no platform is provided, logs out of your Prime account.
Available platforms are: prime, hyper, copilot, openai.`,
	Example: `
# Sign out from your Prime account
prime logout

# Sign out from Hyper
prime logout hyper

# Sign out from GitHub Copilot
prime logout copilot

# Sign out from your ChatGPT (OpenAI) account
prime logout openai
  `,
	ValidArgs: []cobra.Completion{
		"prime",
		"account",
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		"openai",
		"chatgpt",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if len(args) == 0 || args[0] == "prime" || args[0] == "account" {
			svc := account.NewService()
			defer svc.Close()
			return logoutPrimeAccount(getLogoutContext(), svc, cmd.OutOrStdout(), force, logout.Confirm)
		}

		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		progressEnabled := ws.Config.Options.Progress == nil || *ws.Config.Options.Progress
		if progressEnabled && supportsProgressBar() {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
			defer func() { _, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar) }()
		}

		provider := args[0]

		// Canonicalize aliases before prompting.
		switch provider {
		case "hyper":
		case "copilot", "github", "github-copilot":
			provider = "copilot"
		case "openai", "chatgpt":
			provider = "openai"
		default:
			return fmt.Errorf("unknown platform: %s", provider)
		}

		if !force {
			ok, err := logout.Confirm(fmt.Sprintf("Are you sure you want to log out of %s?", providerDisplayNames[provider]))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Logout cancelled.")
				return nil
			}
		}

		switch provider {
		case "hyper":
			return logoutHyper(c, ws.ID)
		case "copilot":
			return logoutCopilot(c, ws.ID)
		case "openai":
			return logoutOpenAI(c, ws.ID)
		default:
			return fmt.Errorf("unknown platform: %s", provider)
		}
	},
}

func logoutPrimeAccount(
	ctx context.Context,
	svc *account.Service,
	out io.Writer,
	force bool,
	confirmFn func(string) (bool, error),
) error {
	if out == nil {
		out = os.Stdout
	}
	if !force && confirmFn != nil {
		ok, err := confirmFn("Are you sure you want to log out of Prime?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "Logout cancelled.")
			return nil
		}
	}
	svc.CancelOrLogout(ctx)
	fmt.Fprintln(out, "Successfully logged out of Prime.")
	return nil
}

func logoutHyper(c *client.Client, wsID string) error {
	ctx := getLogoutContext()

	if err := cmp.Or(
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.api_key"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.oauth"),
	); err != nil {
		return err
	}

	fmt.Printf("Successfully logged out of %s.\n", providerDisplayNames["hyper"])
	return nil
}

func logoutCopilot(c *client.Client, wsID string) error {
	ctx := getLogoutContext()

	if err := cmp.Or(
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.copilot.api_key"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.copilot.oauth"),
	); err != nil {
		return err
	}

	fmt.Printf("Successfully logged out of %s.\n", providerDisplayNames["copilot"])
	return nil
}

func logoutOpenAI(c *client.Client, wsID string) error {
	ctx := getLogoutContext()

	// Logout clears every OpenAI credential: the ChatGPT token and its
	// model catalog, and the API key too. The API key mirrors the access
	// token when it came from the OAuth flow, and an explicit logout
	// should leave nothing behind either way.
	if err := cmp.Or(
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.openai.oauth"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.openai.chatgpt_models"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.openai.api_key"),
	); err != nil {
		return err
	}

	fmt.Printf("Successfully logged out of %s.\n", providerDisplayNames["openai"])
	return nil
}

func init() {
	logoutCmd.Flags().BoolP("force", "f", false, "Skip logout confirmation prompt")
}

func getLogoutContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}
