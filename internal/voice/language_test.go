package voice

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestNormalizeLanguage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", NormalizeLanguage(""))
	require.Equal(t, "", NormalizeLanguage("auto"))
	require.Equal(t, "", NormalizeLanguage("AUTO"))
	require.Equal(t, "ru", NormalizeLanguage("ru"))
	require.Equal(t, "ru", NormalizeLanguage("ru-RU"))
	require.Equal(t, "en", NormalizeLanguage("en_US"))
	require.Equal(t, "uk", NormalizeLanguage("uk-UA"))
	require.Equal(t, "uk", NormalizeLanguage("ua"))
	require.Equal(t, "", NormalizeLanguage("fr"))
	require.Equal(t, "auto", DisplayLanguage(""))
	require.Equal(t, "ru", DisplayLanguage("ru-RU"))
}
