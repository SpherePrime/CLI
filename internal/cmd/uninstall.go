package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/client"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/server"
	"github.com/SpherePrime/CLI/internal/uninstall"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Prime from this computer",
	Long: `Removes the prime executable and the PATH entry the installer added.
With --purge it also deletes global config, data, and cache directories.
Project-level .primerc and .prime folders are left untouched.`,
	Example: `
# Remove just the binary
prime uninstall

# Remove the binary together with config and data
prime uninstall --purge`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}

		purge, _ := cmd.Flags().GetBool("purge")
		yes, _ := cmd.Flags().GetBool("yes")

		var dirs []string
		if purge {
			dirs = append(dirs,
				filepath.Dir(config.GlobalConfig()),
				filepath.Dir(config.GlobalConfigData()),
				config.GlobalCacheDir(),
			)
		}

		if !yes {
			fmt.Fprintf(cmd.OutOrStdout(), "Prime will be removed from:\n  %s\n", exe)
			for _, d := range dirs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", d)
			}
			fmt.Fprint(cmd.OutOrStdout(), "Continue? [y/N] ")
			line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil && line == "" {
				return err
			}
			if !strings.EqualFold(strings.TrimSpace(line), "y") {
				fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
				return nil
			}
		}

		stopServerQuietly(cmd.Context())

		if err := uninstall.Run(uninstall.Options{
			Executable: exe,
			PurgeDirs:  dirs,
			Out:        cmd.OutOrStdout(),
		}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Prime uninstalled. Close and reopen your terminal to refresh PATH.")
		return nil
	},
}

func stopServerQuietly(ctx context.Context) {
	hostURL, err := server.ParseHostURL(clientHost)
	if err != nil {
		return
	}
	c, err := client.NewClient("", hostURL.Scheme, hostURL.Host)
	if err != nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// ShutdownServerIfIdle refuses to stop a server that is still hosting
	// active sessions (a running TUI). The old unconditional
	// ShutdownServer used to kill the shared server even while the TUI
	// was connected, causing the terminal to lose its connection and
	// appear to "shrink" / close.
	_ = c.ShutdownServerIfIdle(shutdownCtx)
}

func init() {
	uninstallCmd.Flags().Bool("purge", false, "Also remove global config, data, and cache directories")
	uninstallCmd.Flags().BoolP("yes", "y", false, "Do not ask for confirmation")
}
