package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func optionsFromShell(t *testing.T, script string) map[string]any {
	t.Helper()

	jsonBytes, err := LoadShellConfig(t.Context(), filepath.Join(t.TempDir(), "primerc"), []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	options, ok := result["options"].(map[string]any)
	require.True(t, ok, "options section missing")
	return options
}

func TestOptionVoice(t *testing.T) {
	t.Parallel()

	options := optionsFromShell(t, `option voice on
option voice language ru
option voice max-duration 60
option voice record-command "rec -q -t raw -"`)

	voice, ok := options["voice"].(map[string]any)
	require.True(t, ok, "voice section missing")

	require.Equal(t, true, voice["enabled"])
	require.Equal(t, "ru", voice["language"])
	require.Equal(t, float64(60), voice["max_duration"])
	require.Equal(t, "rec -q -t raw -", voice["record_command"])
}

func TestOptionVoiceOff(t *testing.T) {
	t.Parallel()

	options := optionsFromShell(t, "option voice off")
	voice := options["voice"].(map[string]any)
	require.Equal(t, false, voice["enabled"])

	options = optionsFromShell(t, "option voice enabled false")
	voice = options["voice"].(map[string]any)
	require.Equal(t, false, voice["enabled"])
}

func TestOptionVoiceWithoutQuotes(t *testing.T) {
	t.Parallel()

	options := optionsFromShell(t, `option voice record-command rec -q -t raw -`)
	voice := options["voice"].(map[string]any)
	require.Equal(t, "rec -q -t raw -", voice["record_command"])
}

func TestOptionVoiceRejectsBadValues(t *testing.T) {
	t.Parallel()

	for _, script := range []string{
		"option voice max-duration zero",
		"option voice max-duration -5",
		"option voice enabled maybe",
		"option voice language",
		"option voice unknown-key value",
	} {
		t.Run(script, func(t *testing.T) {
			_, err := LoadShellConfig(t.Context(), filepath.Join(t.TempDir(), "primerc"), []byte(script))
			require.Error(t, err)
		})
	}
}
