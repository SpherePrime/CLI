package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestResourceOnlineColorDiffersFromOffline(t *testing.T) {
	for _, name := range AllThemeNames() {
		theme := ThemeForName(name)
		require.NotEqual(t, theme.Resource.OfflineIcon.GetForeground(), theme.Resource.OnlineIcon.GetForeground(), name)
	}
}
