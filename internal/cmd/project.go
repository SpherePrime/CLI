package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/SpherePrime/CLI/internal/projects"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Open a saved project",
}

var projectUseCmd = &cobra.Command{
	Use:   "use <path-or-number>",
	Short: "Start Prime in a saved project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := projects.Find(args[0])
		if err != nil {
			return fmt.Errorf("project %q was not found; run `prime projects` to list saved projects", args[0])
		}
		if info, err := os.Stat(project.Path); err != nil || !info.IsDir() {
			return fmt.Errorf("project directory is not available: %s", project.Path)
		}
		if err := projects.Touch(project.Path); err != nil {
			return fmt.Errorf("failed to update project access time: %w", err)
		}

		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to locate prime executable: %w", err)
		}
		child := exec.Command(executable, "--cwd", project.Path)
		child.Stdin = os.Stdin
		child.Stdout = cmd.OutOrStdout()
		child.Stderr = cmd.ErrOrStderr()
		child.Env = os.Environ()
		return child.Run()
	},
}

func init() {
	projectCmd.AddCommand(projectUseCmd)
}
