//go:build windows

package update

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestNewPowershellCmd_NeverRestylesCallerConsole guards against a
// regression where a -WindowStyle switch was passed to powershell.exe.
// PowerShell applies -WindowStyle Hidden via ShowWindow(GetConsoleWindow(),
// SW_HIDE); a console child inherits the caller's console, so that switch
// literally hides the user's own terminal window. Children must instead be
// spawned with CREATE_NO_WINDOW (HideWindow) and no window styling.
func TestNewPowershellCmd_NeverRestylesCallerConsole(t *testing.T) {
	cmd := newPowershellCmd("Write-Output 1")

	for _, arg := range cmd.Args {
		require.NotEqual(t, "-WindowStyle", arg, "restyling the shared console collapses the user's terminal")
	}
	require.NotNil(t, cmd.SysProcAttr)
	require.True(t, cmd.SysProcAttr.HideWindow,
		"child must be created with CREATE_NO_WINDOW so it has no console window of its own")
}
