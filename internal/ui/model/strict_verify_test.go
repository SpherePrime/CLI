package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/attachments"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStatusStrictVerifyBadgeOnlyWhenEnabled(t *testing.T) {
	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.status.helpKm = u
	u.status.SetMode(uiInputModeCode, false)

	u.status.SetStrictVerify(false)
	require.NotContains(t, ansi.Strip(u.status.modeBadge()), "strict")
	require.NotContains(t, ansi.Strip(u.status.modeBadge()), "строго")

	u.status.SetStrictVerify(true)
	badge := ansi.Strip(u.status.modeBadge())
	require.True(t, badge != "" && (containsAny(badge, "strict", "строго")))
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && indexOf(haystack, n) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
