package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwertyfa288/CLI/vendordeps/stretchr/testify/require"
	"github.com/dwertyfa288/CLI/vendordeps/tidwall/gjson"
)

// isolateGlobalDirs points the global config and data locations at scratch
// directories so a test never reads or rewrites the developer's own config.
func isolateGlobalDirs(t *testing.T) (configDir, dataDir string) {
	t.Helper()

	root := t.TempDir()
	configDir = filepath.Join(root, "config", "prime")
	dataDir = filepath.Join(root, "data", "prime")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PRIME_GLOBAL_CONFIG", configDir)
	t.Setenv("PRIME_GLOBAL_DATA", dataDir)
	return configDir, dataDir
}

func readConfigFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, path)
	return string(data)
}

func TestSectionFileForRoutesEachKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"providers.openai.api_key":  providersSectionFile,
		"models.large":              modelsSectionFile,
		"recent_models.small":       modelsSectionFile,
		"mcp.github":                mcpSectionFile,
		"lsp.gopls.command":         lspSectionFile,
		"options.skills_paths":      skillsSectionFile,
		"options.disabled_skills":   skillsSectionFile,
		"options.smart_tools":       optionsSectionFile,
		"options.tui.mouse":         optionsSectionFile,
		"tools.glob":                optionsSectionFile,
		"permissions.allowed_tools": optionsSectionFile,
		"hooks.PreToolUse":          optionsSectionFile,
		"$schema":                   legacyConfigName(),
		"futurekey":                 legacyConfigName(),
	}

	for key, want := range cases {
		require.Equal(t, want, sectionFileFor(key), "key %s", key)
	}
}

// mockProviderConfig is a self-contained provider definition the section tests
// use so reloads resolve a real model instead of failing and keeping the stale
// in-memory config.
const mockProviderConfig = `{
	"options": {"disable_default_providers": true, "disable_provider_auto_update": true},
	"providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
		"base_url": "http://127.0.0.1:9/v1", "api_key": "sk-1",
		"models": [{"id": "m-1", "name": "M-1", "context_window": 8192, "default_max_tokens": 128}]}},
	"models": {"large": {"provider": "mock", "model": "m-1"},
	           "small": {"provider": "mock", "model": "m-1"}}
}`

// TestGlobalSettingsLiveInTheConfigDirectory pins the storage contract: every
// persisted setting ends up under the user config directory, split by section,
// and never in the machine data directory.
func TestGlobalSettingsLiveInTheConfigDirectory(t *testing.T) {
	configDir, dataDir := isolateGlobalDirs(t)

	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, legacyConfigName()),
		[]byte(mockProviderConfig),
		0o600,
	))

	store, err := Load(t.TempDir(), "", false)
	require.NoError(t, err)

	owned := map[string]string{
		"providers.mock.base_url": providersSectionFile,
		"models.large":            modelsSectionFile,
		"mcp.context7":            mcpSectionFile,
		"lsp.gopls":               lspSectionFile,
		"options.skills_paths":    skillsSectionFile,
		"options.smart_tools":     optionsSectionFile,
	}
	fields := map[string]any{
		"providers.mock.base_url": "https://example.test/v1",
		"models.large":            SelectedModel{Provider: "mock", Model: "m-1"},
		"mcp.context7":            MCPConfig{Command: "npx"},
		"lsp.gopls":               LSPConfig{Command: "gopls", FileTypes: []string{"go"}},
		"options.skills_paths":    []string{filepath.Join(t.TempDir(), "skills")},
		"options.smart_tools":     true,
	}
	require.NoError(t, store.SetConfigFields(ScopeGlobal, fields))

	for key, name := range owned {
		path := filepath.Join(configDir, name)
		require.True(t, fileHasContent(path), "%s must be stored in %s", key, name)
		require.True(t, gjson.Get(readConfigFile(t, path), key).Exists(), "key %s", key)
	}

	// Nothing is written next to the machine-owned data file.
	for _, name := range append(sectionFileNames(), legacyConfigName()) {
		require.False(t, fileHasContent(filepath.Join(dataDir, name)), "unexpected file %s", name)
	}

	// The sections are read back, so a reload sees exactly what is on disk.
	require.NoError(t, store.ReloadFromDisk(t.Context()))
	require.True(t, store.Config().Options.SmartTools)
	require.Contains(t, store.Config().Options.SkillsPaths, fields["options.skills_paths"].([]string)[0])
	pc, ok := store.Config().Providers.Get("mock")
	require.True(t, ok)
	require.Equal(t, "https://example.test/v1", pc.BaseURL)
}

// TestMigrateLegacyDataConfigMovesEverySection covers the one-time move: the
// pre-split data-directory file is distributed across the config directory and
// stops being read afterwards.
func TestMigrateLegacyDataConfigMovesEverySection(t *testing.T) {
	configDir, dataDir := isolateGlobalDirs(t)

	legacy := filepath.Join(dataDir, legacyConfigName())
	require.NoError(t, os.WriteFile(legacy, []byte(`{
		"$schema": "https://example.test/schema.json",
		"providers": {"mock": {"api_key": "sk-legacy"}},
		"models": {"large": {"provider": "mock", "model": "m-old"}},
		"mcp": {"context7": {"command": "npx"}},
		"options": {"debug": true, "skills_paths": ["~/skills"]}
	}`), 0o600))

	migrateToSectionFiles()

	require.False(t, fileHasContent(legacy), "the old single file must stop shadowing the sections")
	require.True(t, fileHasContent(legacy+".bak"), "the original is kept as a backup")

	sections := map[string]string{
		"providers.mock.api_key": providersSectionFile,
		"models.large.model":     modelsSectionFile,
		"mcp.context7":           mcpSectionFile,
		"options.debug":          optionsSectionFile,
		"options.skills_paths":   skillsSectionFile,
	}
	for key, name := range sections {
		path := filepath.Join(configDir, name)
		require.True(t, gjson.Get(readConfigFile(t, path), key).Exists(), "key %s in %s", key, name)
	}

	// Re-running the migration changes nothing.
	migrateToSectionFiles()
	require.Equal(t, "sk-legacy", gjson.Get(
		readConfigFile(t, filepath.Join(configDir, providersSectionFile)),
		"providers.mock.api_key",
	).String())
}

// TestMigrateGlobalConfigKeepsSchemaPointer checks the hand-written global file
// is reduced to keys that have no section of their own instead of being moved.
func TestMigrateGlobalConfigKeepsSchemaPointer(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)

	anchor := filepath.Join(configDir, legacyConfigName())
	require.NoError(t, os.WriteFile(anchor, []byte(`{
		"$schema": "https://example.test/schema.json",
		"providers": {"mock": {"api_key": "sk-hand"}}
	}`), 0o600))

	migrateToSectionFiles()

	remaining := readConfigFile(t, anchor)
	require.Contains(t, remaining, "$schema", "editors still resolve the schema")
	require.NotContains(t, remaining, "providers", "migrated keys must not shadow the sections")
	require.Equal(t, "sk-hand", gjson.Get(
		readConfigFile(t, filepath.Join(configDir, providersSectionFile)),
		"providers.mock.api_key",
	).String())
	require.True(t, fileHasContent(anchor+".bak"), "the original is kept as a backup")
}

// TestWorkspaceConfigIsAlsoSplitBySection covers project-scoped writes: the
// project's data directory holds one file per section, and a reload reads them
// back on top of the global config.
func TestWorkspaceConfigIsAlsoSplitBySection(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	workingDir := t.TempDir()

	// A global value the project is going to override.
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, optionsSectionFile),
		[]byte(`{"options":{"smart_tools":true,"debug":true}}`),
		0o600,
	))

	store, err := Load(workingDir, "", false)
	require.NoError(t, err)
	require.True(t, store.Config().Options.SmartTools)

	require.NoError(t, store.SetConfigField(ScopeWorkspace, "options.smart_tools", false))
	require.NoError(t, store.SetConfigField(ScopeWorkspace, "mcp.context7", MCPConfig{Command: "npx"}))

	wsDir := filepath.Join(workingDir, ".prime")
	require.True(t, fileHasContent(filepath.Join(wsDir, optionsSectionFile)))
	require.True(t, fileHasContent(filepath.Join(wsDir, mcpSectionFile)))

	require.NoError(t, store.ReloadFromDisk(t.Context()))
	require.False(t, store.Config().Options.SmartTools, "the project file overrides the global one")
	require.True(t, store.Config().Options.Debug, "keys the project does not set keep their global value")
	_, hasMCP := store.Config().MCP["context7"]
	require.True(t, hasMCP)
}

// TestHandEditedLegacyFileStillApplies protects the upgrade path: keys added to
// prime.json after the split still reach the sections on the next load.
func TestHandEditedLegacyFileStillApplies(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)
	providers := filepath.Join(configDir, providersSectionFile)

	anchor := filepath.Join(configDir, legacyConfigName())
	require.NoError(t, os.WriteFile(anchor, []byte(`{"providers":{"mock":{"api_key":"sk-a"}}}`), 0o600))
	migrateToSectionFiles()
	require.Equal(t, "sk-a", gjson.Get(readConfigFile(t, providers), "providers.mock.api_key").String())

	require.NoError(t, os.WriteFile(anchor, []byte(`{"providers":{"mock":{"api_key":"sk-b"}}}`), 0o600))
	migrateToSectionFiles()
	require.Equal(t, "sk-b", gjson.Get(readConfigFile(t, providers), "providers.mock.api_key").String())
}

// TestSplitSectionFilesStayValidJSON guards against a migration writing a
// partial document: each section must parse on its own.
func TestSplitSectionFilesStayValidJSON(t *testing.T) {
	configDir, _ := isolateGlobalDirs(t)

	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, legacyConfigName()),
		[]byte(`{"models":{"large":{"provider":"mock","model":"m-1"}},"options":{"debug":true,"skills_paths":["~/s"]}}`),
		0o600,
	))

	migrateToSectionFiles()

	for _, path := range configFilesForDir(configDir) {
		if !fileHasContent(path) {
			continue
		}
		var doc map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(readConfigFile(t, path)), &doc), "file %s", path)
	}
}
