package model

import (
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestKeyBindingsRemapByName(t *testing.T) {
	km := DefaultKeyMap()

	require.NoError(t, firstProblem(ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "ctrl+alt+n",
	})))

	require.Equal(t, []string{"ctrl+alt+n"}, km.Chat.NewSession.Keys())
}

// The name in the config is the dotted field path, so a typo has to be reported
// rather than ignored: a remap that silently does nothing is worse than no
// remap, because the user believes a key was moved and it was not.
func TestKeyBindingsReportUnknownName(t *testing.T) {
	km := DefaultKeyMap()
	before := km.Chat.NewSession.Keys()

	problems := ApplyKeyBindings(&km, map[string]string{
		"chat.new_sessions": "ctrl+alt+n",
	})

	require.Len(t, problems, 1)
	require.Contains(t, problems[0], "chat.new_sessions")
	require.Equal(t, before, km.Chat.NewSession.Keys(),
		"a name that matches nothing must leave the defaults alone")
}

func TestKeyBindingsUnknownNameIsReportedEvenWithValidOnes(t *testing.T) {
	km := DefaultKeyMap()

	problems := ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "ctrl+alt+n",
		"nope.nope":       "ctrl+alt+x",
	})

	require.Len(t, problems, 1)
	require.Contains(t, problems[0], "nope.nope")
	require.Equal(t, []string{"ctrl+alt+n"}, km.Chat.NewSession.Keys(),
		"a good entry next to a bad one still has to apply")
}

// The help line is what tells the user what to press. Leaving it on the old key
// means the UI advertises a binding that no longer works.
func TestKeyBindingsUpdateTheHelpKey(t *testing.T) {
	km := DefaultKeyMap()

	require.NoError(t, firstProblem(ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "ctrl+alt+n",
	})))

	help := km.Chat.NewSession.Help()
	require.Equal(t, "ctrl+alt+n", help.Key)
	require.NotEmpty(t, help.Desc, "the description must survive the remap")
}

func TestKeyBindingsAcceptSeveralKeys(t *testing.T) {
	km := DefaultKeyMap()

	require.NoError(t, firstProblem(ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "ctrl+alt+n, ctrl+alt+shift+n",
	})))

	require.Equal(t, []string{"ctrl+alt+n", "ctrl+alt+shift+n"}, km.Chat.NewSession.Keys())
}

// An empty value is how an action gets disabled without deleting its name, so
// the same map can both rebind and unbind.
func TestKeyBindingsEmptyValueUnbinds(t *testing.T) {
	km := DefaultKeyMap()

	require.NoError(t, firstProblem(ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "",
	})))

	require.False(t, km.Chat.NewSession.Enabled())
	require.Empty(t, km.Chat.NewSession.Keys())
}

// Bubbles does not resolve a key bound to two actions: both match, and whichever
// case the switch reaches first wins, so the other action stops responding with
// no error anywhere. Worth saying out loud.
func TestKeyBindingsReportConflicts(t *testing.T) {
	km := DefaultKeyMap()

	problems := ApplyKeyBindings(&km, map[string]string{
		"chat.new_session": "ctrl+d",
	})

	require.Len(t, problems, 1)
	require.Contains(t, problems[0], "ctrl+d")
	require.Contains(t, problems[0], "chat.new_session")
	require.Contains(t, problems[0], "chat.details")
}

func TestKeyBindingsNoOverridesIsAQuietNoop(t *testing.T) {
	km := DefaultKeyMap()
	before := km.Chat.NewSession.Keys()

	require.Empty(t, ApplyKeyBindings(&km, nil))
	require.Equal(t, before, km.Chat.NewSession.Keys())
}

// The names are derived from the keymap's own fields, so a binding added in a
// later version is remappable the moment it lands, with nobody remembering to
// add it to a table.
func TestKeyBindingNamesCoverTheWholeKeyMap(t *testing.T) {
	names := KeyBindingNames(DefaultKeyMap())
	require.NotEmpty(t, names)

	joined := ""
	for _, n := range names {
		joined += n + "\n"
	}

	for _, expected := range []string{
		"chat.new_session",
		"chat.toggle_thinking",
		"editor.send_message",
		"editor.paste_image",
		"quit",
		"commands",
	} {
		require.Contains(t, joined, expected,
			"every binding must be reachable by name")
	}
}

func TestSnakeCaseFieldNames(t *testing.T) {
	for input, expected := range map[string]string{
		"NewSession":      "new_session",
		"PasteImage":      "paste_image",
		"Quit":            "quit",
		"ClearHighlight":  "clear_highlight",
		"HTTPServer":      "http_server",
		"SelectAll":       "select_all",
		"AddAttachment":   "add_attachment",
	} {
		require.Equal(t, expected, snakeCase(input), input)
	}
}

// firstProblem returns the first reported problem as an error, or nil when there
// were none, so a table of good inputs reads without a helper at every step.
func firstProblem(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(problems[0])
}