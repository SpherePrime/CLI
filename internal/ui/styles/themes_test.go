package styles

import (
	"fmt"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestNamedThemesAreDistinctAndComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range AllThemeNames() {
		theme := ThemeForName(name)
		key := fmt.Sprint(theme.Tool.NameNormal.GetForeground(), theme.ANSI)
		require.False(t, seen[key], "theme %s duplicates another palette", name)
		seen[key] = true
		require.NotEmpty(t, theme.Iridescence)
	}
	require.GreaterOrEqual(t, len(seen), 15)
}

func TestThemeNameFallbackAndAliases(t *testing.T) {
	require.Equal(t, "default", ThemeKey(""))
	require.Equal(t, "default", ThemeKey("unknown"))
	require.Equal(t, "charcoal", ThemeKey("oled"))
	require.Equal(t, "default", ThemeKey("panthera"))
	require.Equal(t, "nord", ThemeKey(" Nord "))
}

func TestEveryDesignFramesTheHeader(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "focus", "dashboard", "terminal", "studio", "opencode", "paper", "blueprint", "ember", "neon"} {
		t.Run(design, func(t *testing.T) {
			style := ApplyDesign(ColorTonePantera(), design).DesignHeader
			_, top, right, bottom, left := style.GetBorder()
			require.True(t, top)
			require.True(t, right)
			require.True(t, bottom)
			require.True(t, left)
		})
	}
}
