package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/i18n"
	ui "github.com/SpherePrime/CLI/internal/ui/model"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// prime keys is the only place a user learns the names that go into
// options.keybindings. A name it prints wrongly is a binding that silently
// does nothing, reported as an unknown name at startup - so the names worth
// pinning are the ones the command's own help text uses as examples.
//
// Each line is a name padded out to a column followed by the keys it is bound
// to, so the name is the first field rather than the whole line - which is
// what the help means by "the names in the left column".
func TestKeysCommandPrintsBindableNames(t *testing.T) {
	var buf bytes.Buffer
	keysCmd.SetOut(&buf)
	require.NoError(t, keysCmd.RunE(keysCmd, nil), "prime keys must not fail")

	out := buf.String()
	for _, documented := range []string{"chat.new_session", "editor.send_message", "chat.toggle_thinking"} {
		require.Contains(t, out, documented,
			"a name the help text tells people to use must be printed by the command that lists them")
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.NotEmpty(t, lines[0], "prime keys printed nothing")

	// Duplicates of the left column would be ambiguous: the config is a map
	// keyed by name, so a name appearing twice means two actions answer to it.
	seen := map[string]bool{}
	for _, line := range lines {
		require.NotEmpty(t, strings.TrimSpace(line), "no blank lines in the listing")
		fields := strings.Fields(line)
		require.NotEmpty(t, fields, "a line with nothing on it: %q", line)
		name := fields[0]
		require.False(t, seen[name], "duplicate entry %q", name)
		seen[name] = true
	}
}

// The listing is the defaults, built fresh with the English catalogue, so
// what it prints must be exactly what the keymap builder says - not a second
// hand-maintained copy of the same list that could drift from it.
func TestKeysCommandPrintsExactlyWhatTheKeymapHolds(t *testing.T) {
	var buf bytes.Buffer
	keysCmd.SetOut(&buf)
	require.NoError(t, keysCmd.RunE(keysCmd, nil))

	printed := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	want := ui.KeyBindingNames(ui.BuildKeyMap(i18n.New(i18n.En)))
	require.Equal(t, want, printed,
		"the command must print the keymap's own names, in order")
}
