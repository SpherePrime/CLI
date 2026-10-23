package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProviderTransactionRollsBackExistingAndNewFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a-existing.json")
	created := filepath.Join(dir, "b-created.json")
	failed := filepath.Join(dir, "c-failed.json")
	original := []byte(`{"secret":"preserved","session_models":{"chat":{"provider":"old"}}}`)
	require.NoError(t, os.WriteFile(existing, original, 0600))
	failure := errors.New("fixture write failed")
	err := persistProviderFiles(map[string][]byte{existing: original}, map[string][]byte{
		existing: []byte(`{"changed":true}`),
		created:  []byte(`{"new":true}`),
		failed:   []byte(`{"failure":true}`),
	}, func(path string, data []byte, mode os.FileMode) error {
		if path == failed {
			return failure
		}
		return atomicWriteFile(path, data, mode)
	})
	require.ErrorIs(t, err, failure)
	data, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(data))
	_, err = os.Stat(created)
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(failed)
	require.True(t, os.IsNotExist(err))
}

func TestGlobalProviderRenameRejectsWorkspaceShadowBeforeWriting(t *testing.T) {
	globalDir, workspaceDir := t.TempDir(), t.TempDir()
	source := &ConfigStore{config: &Config{}, globalDataPath: filepath.Join(globalDir, "prime.json")}
	workspace := &ConfigStore{config: &Config{}, workspacePath: filepath.Join(workspaceDir, "prime.json")}
	original := []byte(`{"providers":{"old":{"id":"old","api_key":"preserve-secret"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, providersSectionFile), original, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, providersSectionFile), original, 0600))
	err := source.SaveProviderWithWorkspaces(ScopeGlobal, "old", providerEditFixture(t, "renamed"), nil, []*ConfigStore{workspace})
	require.ErrorContains(t, err, "сохранены настройки этого провайдера")
	for _, dir := range []string{globalDir, workspaceDir} {
		data, err := os.ReadFile(filepath.Join(dir, providersSectionFile))
		require.NoError(t, err)
		require.Equal(t, original, data)
	}
	shadow := []byte(`{"providers":{"renamed":{"api_key":"workspace-secret"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, providersSectionFile), shadow, 0600))
	err = source.SaveProviderWithWorkspaces(ScopeGlobal, "old", providerEditFixture(t, "renamed"), nil, []*ConfigStore{workspace})
	require.ErrorContains(t, err, "новым ID уже существует")
	data, err := os.ReadFile(filepath.Join(globalDir, providersSectionFile))
	require.NoError(t, err)
	require.Equal(t, original, data)
	data, err = os.ReadFile(filepath.Join(workspaceDir, providersSectionFile))
	require.NoError(t, err)
	require.Equal(t, shadow, data)
}
