package tools

import (
	"bytes"
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestBlockedRootHint(t *testing.T) {
	restore := sudoToolAvailable
	t.Cleanup(func() { sudoToolAvailable = restore })

	sudoToolAvailable = true
	err := errors.New(`command is not allowed for security reasons: "sudo"`)
	require.Contains(t, blockedRootHint(err), "sudo tool")

	sudoToolAvailable = false
	require.Empty(t, blockedRootHint(err))

	require.Empty(t, blockedRootHint(nil))
	require.Empty(t, blockedRootHint(errors.New("exit code 1")))
}

func TestBashDescriptionSudoMention(t *testing.T) {
	render := func(sudoAvailable bool) string {
		var out bytes.Buffer
		err := bashDescriptionTpl.Execute(&out, bashDescriptionData{
			BannedCommands:  "sudo",
			MaxOutputLength: MaxOutputLength,
			SudoAvailable:   sudoAvailable,
			Platform:        "linux",
		})
		require.NoError(t, err)
		return out.String()
	}

	withSudo := render(true)
	require.Contains(t, withSudo, "The sudo tool IS available")
	require.Contains(t, withSudo, "NEVER ask the user to run sudo or root commands manually")

	withoutSudo := render(false)
	require.NotContains(t, withoutSudo, "The sudo tool IS available")
	require.Contains(t, withoutSudo, "cannot be run elevated")
}
