package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwertyfa288/CLI/internal/update"
	"github.com/dwertyfa288/CLI/internal/version"
	"github.com/dwertyfa288/CLI/vendordeps/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install a new version of Prime",
	Example: `
# Check and install with a prompt
prime update

# Install without asking
prime update --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		out := cmd.OutOrStdout()

		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}

		ctx := cmd.Context()
		info, err := update.Check(ctx, version.Version, update.Default)
		if err != nil {
			return err
		}
		if !info.Available() {
			fmt.Fprintf(out, "Prime is up to date (v%s).\n", info.Current)
			return nil
		}
		fmt.Fprintf(out, "Prime v%s is available, you have v%s.\n", info.Latest, info.Current)

		if !yes {
			fmt.Fprint(out, "Update now? [y/N] ")
			line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil && line == "" {
				return err
			}
			if !strings.EqualFold(strings.TrimSpace(line), "y") {
				fmt.Fprintln(out, "cancelled")
				return nil
			}
		}

		stopServerQuietly(ctx)

		if _, err := update.Install(ctx, version.Version, exe, update.Default, out); err != nil {
			return err
		}
		fmt.Fprintf(out, "Updated to v%s. Restart Prime to run the new version.\n", info.Latest)
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolP("yes", "y", false, "Do not ask for confirmation")
}
