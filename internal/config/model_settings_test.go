package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// newGlobalStore returns a store whose global section files land in dir.
func newGlobalStore(dir string) *ConfigStore {
	return &ConfigStore{
		config:         &Config{Models: map[SelectedModelType]SelectedModel{}},
		globalDataPath: filepath.Join(dir, "prime.json"),
	}
}

func TestUpdatePreferredModelKeepsSettingsAcrossSwitches(t *testing.T) {
	t.Parallel()

	store := newGlobalStore(t.TempDir())

	base := SelectedModel{Provider: "acme", Model: "big"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, base))

	configured := base
	configured.ContextWindow = 250000
	configured.PriceIn = 1.5
	configured.PriceOut = 6
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, configured))

	// Model switches rebuild the selection from the catalog and carry no
	// overrides, so returning to a model must restore what was stored.
	other := SelectedModel{Provider: "acme", Model: "other"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, other))
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, base))

	restored := store.Config().Models[SelectedModelTypeLarge]
	require.Equal(t, "big", restored.Model)
	require.EqualValues(t, 250000, restored.ContextWindow)
	require.InDelta(t, 1.5, restored.PriceIn, 1e-9)
	require.InDelta(t, 6.0, restored.PriceOut, 1e-9)
}

func TestModelSettingsPersistToDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newGlobalStore(dir)

	configured := SelectedModel{Provider: "acme", Model: "big", ContextWindow: 1234}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, configured))

	data, err := os.ReadFile(filepath.Join(dir, modelsSectionFile))
	require.NoError(t, err)

	var onDisk struct {
		Models        map[SelectedModelType]SelectedModel `json:"models"`
		ModelSettings map[string]ModelSettings            `json:"model_settings"`
	}
	require.NoError(t, json.Unmarshal(data, &onDisk))

	settings, ok := onDisk.ModelSettings[modelSettingsKey("acme", "big")]
	require.True(t, ok, "expected a model_settings entry, file was: %s", data)
	require.EqualValues(t, 1234, settings.ContextWindow)
}

func TestApplyModelSettingsFillsUnsetOverrides(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		ModelSettings: map[string]ModelSettings{
			modelSettingsKey("acme", "big"): {ContextWindow: 999, PriceIn: 2},
		},
	}

	model := SelectedModel{Provider: "acme", Model: "big"}
	require.True(t, cfg.applyModelSettings(&model))
	require.EqualValues(t, 999, model.ContextWindow)
	require.InDelta(t, 2.0, model.PriceIn, 1e-9)

	// An explicit value on the selection wins over the stored one.
	explicit := SelectedModel{Provider: "acme", Model: "big", ContextWindow: 10}
	require.True(t, cfg.applyModelSettings(&explicit))
	require.EqualValues(t, 10, explicit.ContextWindow)

	unknown := SelectedModel{Provider: "acme", Model: "other"}
	require.False(t, cfg.applyModelSettings(&unknown))
}

func TestConfigJSONIsIndentedOnDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newGlobalStore(dir)

	require.NoError(t, store.SetConfigField(ScopeGlobal, "options.language", "ru"))

	data, err := os.ReadFile(filepath.Join(dir, optionsSectionFile))
	require.NoError(t, err)

	require.Greater(t, len(strings.Split(strings.TrimSpace(string(data)), "\n")), 1,
		"config file should span multiple lines, got: %s", data)
	require.Contains(t, string(data), "\n  \"options\"")
	require.Equal(t, byte('\n'), data[len(data)-1], "file should end with a newline")
	require.True(t, json.Valid(data))
}

func TestFormatConfigJSONUnescapesHTMLEntities(t *testing.T) {
	t.Parallel()

	out := formatConfigJSON("providers.json",
		[]byte(`{"providers":{"a":{"base_url":"https://x.test/v1?a=1&b=2"}}}`))

	require.Contains(t, string(out), "a=1&b=2")
	require.NotContains(t, string(out), `\u0026`)
	require.True(t, json.Valid(out))
}

func TestFormatConfigJSONLeavesOtherContentAlone(t *testing.T) {
	t.Parallel()

	original := []byte("not json at all")
	require.Equal(t, original, formatConfigJSON("primerrc", original))
	require.Equal(t, original, formatConfigJSON("broken.json", original))
}
