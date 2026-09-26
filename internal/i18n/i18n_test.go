package i18n

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestKeyParity(t *testing.T) {
	en := Catalogs[En]
	ru := Catalogs[Ru]
	var missing []string
	for k := range en.Strings {
		if _, ok := ru.Strings[k]; !ok {
			missing = append(missing, k)
		}
	}
	// Reverse parity is useful too: keys only in Russian are typos.
	for k := range ru.Strings {
		if _, ok := en.Strings[k]; !ok {
			missing = append(missing, "en:"+k)
		}
	}
	require.Empty(t, missing, "catalogs out of sync")
}

func TestNewFallsBackToEnglish(t *testing.T) {
	tr := New("xx")
	require.Equal(t, En, tr.Locale())
	require.Equal(t, "New Session", tr.Label("cmd.new_session"))
}

func TestSprintf(t *testing.T) {
	tr := New(Ru)
	require.Equal(t, "Изменённые файлы", tr.Label("sidebar.modified_files"))
	require.Equal(t, "…ещё 3", tr.Sprintf("sidebar.more", 3))
	require.Equal(t, "Нет", tr.Label("btn.no"))
}
