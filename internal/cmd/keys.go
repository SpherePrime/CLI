package cmd

import (
	"github.com/SpherePrime/CLI/internal/i18n"
	ui "github.com/SpherePrime/CLI/internal/ui/model"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List the keybindings and how to rebind them",
	Long: `List every action Prime can bind a key to, and the key it uses by default.

The names in the left column go in options.keybindings in your config to
rebind an action:

  {
    "options": {
      "keybindings": {
        "chat.new_session": "ctrl+alt+n",
        "editor.send_message": "alt+enter,ctrl+s"
      }
    }
  }

A value may list several keys separated by a comma. An empty value unbinds
the action so nothing triggers it. Names that do not match an action are
reported at startup rather than ignored, and so are keys that end up bound to
two actions.`,
	Example: `
# Show every action and its current binding
prime keys
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Built without user overrides on purpose: this lists what the
		// defaults are, which is the thing you need in order to choose what
		// to change. The active keymap already has the remaps applied, and
		// its own name would be the answer to a different question.
		names := ui.KeyBindingNames(ui.BuildKeyMap(i18n.New(i18n.En)))
		for _, n := range names {
			cmd.Println(n)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(keysCmd)
}