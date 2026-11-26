package voice

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestNormalizeLanguage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ru", NormalizeLanguage(""))
	require.Equal(t, "auto", NormalizeLanguage("auto"))
	require.Equal(t, "auto", NormalizeLanguage("AUTO"))
	require.Equal(t, "ru", NormalizeLanguage("ru"))
	require.Equal(t, "ru", NormalizeLanguage("ru-RU"))
	require.Equal(t, "en", NormalizeLanguage("en_US"))
	require.Equal(t, "uk", NormalizeLanguage("uk-UA"))
	require.Equal(t, "uk", NormalizeLanguage("ua"))
	require.Equal(t, "ru", NormalizeLanguage("fr"))
	require.Equal(t, "ru", DisplayLanguage(""))
	require.Equal(t, "auto", DisplayLanguage("auto"))
	require.Equal(t, "ru", DisplayLanguage("ru-RU"))
}
