package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"github.com/SpherePrime/CLI/vendordeps/tidwall/gjson"
)

// The Web Speech endpoint obeys values that are not language tags, so a tag
// Prime cannot read is a tag Prime must not trust to name what the user speaks.
func TestUsableLanguageTag(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		tag  string
		want bool
	}{
		"unset":                             {tag: "", want: true},
		"auto":                              {tag: "auto", want: true},
		"padded auto":                       {tag: "  auto  ", want: true},
		"plain":                             {tag: "ru", want: true},
		"regional":                          {tag: "ru-RU", want: true},
		"upper case":                        {tag: "RU-RU", want: true},
		"script":                            {tag: "zh-Hans-CN", want: true},
		"three letter with no shorter form": {tag: "haw", want: true},
		// So is a legacy spelling of a language the search would not find:
		// dropping it would trade a right answer for two wrong ones.
		"legacy arabic": {tag: "ara", want: true},
		// These are the ones that break: the endpoint reads them as a language
		// the user never asked for instead of refusing them.
		"legacy english":  {tag: "eng", want: false},
		"legacy russian":  {tag: "rus", want: false},
		"english name":    {tag: "english", want: false},
		"underscore":      {tag: "en_US", want: false},
		"unknown code":    {tag: "zz", want: false},
		"private use":     {tag: "qq", want: false},
		"legacy regional": {tag: "eng-US", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.want, usableLanguageTag(testCase.tag))
		})
	}
}

// A value Prime cannot read becomes "auto", so dictation picks a language
// instead of being pinned to the one the endpoint guessed.
func TestMigrateVoiceLanguageFallsBackToAuto(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	section := filepath.Join(configDir, "options.json")
	writeVoiceLanguage(t, section, "eng")

	migrateVoiceLanguage()

	require.Equal(t, voiceLanguageAuto, readVoiceLanguage(t, section))
}

// Running again has to change nothing: the value the migration writes is one it
// accepts, so a second load does not keep rewriting the file.
func TestMigrateVoiceLanguageRunsOnce(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	section := filepath.Join(configDir, "options.json")
	writeVoiceLanguage(t, section, "eng")

	migrateVoiceLanguage()
	first, err := os.ReadFile(section)
	require.NoError(t, err)

	migrateVoiceLanguage()
	second, err := os.ReadFile(section)
	require.NoError(t, err)
	require.Equal(t, string(first), string(second))
}

// A language the user did mean is not Prime's to undo, so nothing is written.
func TestMigrateVoiceLanguageKeepsUsableValue(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	section := filepath.Join(configDir, "options.json")
	writeVoiceLanguage(t, section, "ru-RU")

	migrateVoiceLanguage()

	require.Equal(t, "ru-RU", readVoiceLanguage(t, section))
}

// A stale copy in the catch-all file shadows the section file, so the
// replacement has to land in the section and the copy has to go.
func TestMigrateVoiceLanguageClearsShadowingCopy(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	catchAll := filepath.Join(configDir, legacyConfigName())
	section := filepath.Join(configDir, "options.json")
	writeVoiceLanguage(t, catchAll, "eng")
	require.NoError(t, os.WriteFile(section,
		[]byte(`{"options":{"notifications":"on"}}`), 0o600))

	migrateVoiceLanguage()

	require.Equal(t, voiceLanguageAuto, readVoiceLanguage(t, section))
	data, err := os.ReadFile(catchAll)
	require.NoError(t, err)
	require.False(t, gjson.Get(string(data), "options.voice.language").Exists(),
		"a leftover copy shadows the section it belongs to")
}

// A config that says nothing about dictation is left alone, so enabling the
// plugin later still starts from the search.
func TestMigrateVoiceLanguageLeavesMissingValue(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	section := filepath.Join(configDir, "options.json")
	require.NoError(t, os.WriteFile(section,
		[]byte(`{"options":{"notifications":"on"}}`), 0o600))

	migrateVoiceLanguage()

	data, err := os.ReadFile(section)
	require.NoError(t, err)
	require.False(t, gjson.Get(string(data), "options.voice.language").Exists())
}

// The migration is wired into the load, not just callable: a user only ever
// meets it through starting Prime.
func TestLoadMigratesVoiceLanguage(t *testing.T) {
	configDir, dataDir := isolateGlobalDirs(t)
	section := filepath.Join(configDir, "options.json")
	writeVoiceLanguage(t, section, "eng")

	store, err := Load(t.TempDir(), dataDir, false)
	require.NoError(t, err)
	require.NotNil(t, store)

	require.Equal(t, voiceLanguageAuto, readVoiceLanguage(t, section))
}

// writeVoiceLanguage writes a config file whose only content is the setting.
func writeVoiceLanguage(t *testing.T, path, language string) {
	t.Helper()
	body := `{"options":{"voice":{"language":` + strconv.Quote(language) + `}}}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	requireValidJSON(t, path)
}

// readVoiceLanguage reads the setting back out of a config file.
func readVoiceLanguage(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	requireValidJSON(t, path)
	return gjson.Get(string(data), "options.voice.language").String()
}

// requireValidJSON guards the assertions: gjson reads a truncated document
// without complaining, so a test can pass against a file Prime would refuse to
// load.
func requireValidJSON(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, json.Valid(data),
		"%s is not valid JSON: %q", filepath.Base(path), string(data))
}
