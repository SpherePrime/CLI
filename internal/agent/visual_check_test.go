package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestVisualCheckBlockFollowsImageSupport(t *testing.T) {
	withVision := visualCheckBlock(true)
	require.Contains(t, withVision, "This model can see images")
	require.Contains(t, withVision, "never replaces any of its steps")
	require.Contains(t, withVision, "take new screenshots and look again")
	require.Contains(t, withVision, "Overview first, then close-ups")
	require.Contains(t, withVision, "Interact, do not only look")
	require.Contains(t, withVision, "Prefer the browser")
	require.Contains(t, withVision, "no review agent approved")

	require.Empty(t, visualCheckBlock(false))
}
