package cmd

import (
	"fmt"
	"os"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/mcps"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Prime's built-in MCP servers",
	Long: `Prime ships its own MCP servers inside the same binary. Nothing runs by
default: installing one from the /mcp menu writes an mcp config section that
launches it with "prime mcp serve <name>".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the MCP servers Prime can install",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, server := range mcps.Servers() {
			cmd.Printf("%-8s %s\n", server.Name, server.Title)
			cmd.Printf("%-8s   tools: %v\n", "", server.Tools)
			if installed, ok := mcpSectionEnabled(server.Name); ok {
				state := "installed"
				if installed.Disabled {
					state = "installed (disabled)"
				}
				cmd.Printf("%-8s   %s\n", "", state)
			} else {
				cmd.Printf("%-8s   not installed (use the /mcp menu in the TUI)\n", "")
			}
		}
		return nil
	},
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve <name>",
	Short: "Run a built-in MCP server over stdio",
	Long: `Start one of Prime's built-in MCP servers on stdin/stdout. This is the
command Prime's own mcp config uses; run it manually only for debugging.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, ok := mcps.Lookup(args[0])
		if !ok {
			return fmt.Errorf("unknown built-in mcp server %q (try: prime mcp list)", args[0])
		}
		return server.Serve(commandContext(cmd))
	},
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}

// mcpSectionEnabled reports whether the user's config already carries the
// mcp section this server installs under.
func mcpSectionEnabled(name string) (config.MCPConfig, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return config.MCPConfig{}, false
	}
	store, err := config.Init(cwd, "", false)
	if err != nil {
		return config.MCPConfig{}, false
	}
	cfg := store.Config()
	if cfg == nil || cfg.MCP == nil {
		return config.MCPConfig{}, false
	}
	entry, ok := cfg.MCP[name]
	return entry, ok
}
