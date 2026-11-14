package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/login"
	"github.com/SpherePrime/CLI/internal/oauth/copilot"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Prime to a platform",
	Long: `Login to your Prime account or a specified model platform.
If no platform is provided, authenticates your Prime account via Google.
Available platforms are: prime, hyper, copilot, openai.`,
	Example: `
# Authenticate your Prime account
prime login

# Authenticate with Hyper
prime login hyper

# Authenticate with GitHub Copilot
prime login copilot

# Authenticate with a ChatGPT (OpenAI) account
prime login openai

# Force re-authentication even if already logged in
prime login -f
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
		provider := "prime"
		if len(args) > 0 {
			provider = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")

		if provider == "prime" || provider == "account" {
			svc := account.NewService()
			defer svc.Close()
			return loginPrimeAccount(getLoginContext(), svc, cmd.OutOrStdout(), force)
		}

		ws, cleanup, err := setupWorkspaceWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		switch provider {
		case "hyper":
			return loginHyper(ws, force)
		case "copilot", "github", "github-copilot":
			return loginCopilot(ws, force)
		case "openai", "chatgpt":
			return loginOpenAI(ws, force)
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginPrimeAccount(ctx context.Context, svc *account.Service, out io.Writer, force bool) error {
	if out == nil {
		out = os.Stdout
	}
	if !force {
		st := svc.Restore(ctx)
		if st.SignedIn {
			if st.Email != "" {
				fmt.Fprintf(out, "You are already logged in to Prime as %s.\n", st.Email)
			} else {
				fmt.Fprintln(out, "You are already logged in to Prime.")
			}
			fmt.Fprintln(out, "Use --force to re-authenticate.")
			return nil
		}
	}

	st, err := svc.StartLogin(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Open this URL in your browser to sign in to Prime:\n\n  %s\n\nWaiting for confirmation...\n", st.LoginURL)
	if st.Error != "" {
		fmt.Fprintf(out, "Note: %s\n", st.Error)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			cur := svc.State(ctx)
			if cur.SignedIn {
				if cur.Email != "" {
					fmt.Fprintf(out, "\nYou're now signed in to Prime as %s!\n", cur.Email)
				} else {
					fmt.Fprintln(out, "\nYou're now signed in to Prime!")
				}
				return nil
			}
			if !cur.Waiting {
				if cur.Error != "" {
					return errors.New(cur.Error)
				}
				return errors.New("authentication failed")
			}
		}
	}
}

func init() {
	loginCmd.Flags().BoolP("force", "f", false, "Force re-authentication even if already logged in")
}

func loginHyper(ws workspace.Workspace, force bool) error {
	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("hyper"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to Hyper.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	ctx := getLoginContext()
	token, err := login.Run(ctx, login.PlatformHyper)
	if err != nil {
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "hyper", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(ws workspace.Workspace, force bool) error {
	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("copilot"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to GitHub Copilot.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	ctx := getLoginContext()

	if diskToken, hasDiskToken := copilot.RefreshTokenFromDisk(); hasDiskToken {
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")
		token, err := copilot.RefreshToken(ctx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		if err := ws.SetProviderAPIKey(config.ScopeGlobal, "copilot", token); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("You're now authenticated with GitHub Copilot!")
		return nil
	}

	token, err := login.Run(ctx, login.PlatformCopilot)
	if err != nil {
		if errors.Is(err, copilot.ErrNotAvailable) {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "copilot", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func loginOpenAI(ws workspace.Workspace, force bool) error {
	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("openai"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to OpenAI with a ChatGPT account.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	ctx := getLoginContext()
	token, err := login.Run(ctx, login.PlatformOpenAI)
	if err != nil {
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "openai", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with your ChatGPT account!")
	return nil
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ctx
}
