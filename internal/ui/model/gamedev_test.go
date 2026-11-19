package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/attachments"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStatusGameDevBadgeOnlyWhenEnabled(t *testing.T) {
	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.status.helpKm = u
	u.status.SetMode(uiInputModeCode, false)

	u.status.SetGameDev(false)
	require.NotContains(t, ansi.Strip(u.status.modeBadge()), "gamedev")

	u.status.SetGameDev(true)
	require.Contains(t, ansi.Strip(u.status.modeBadge()), "gamedev")
}
