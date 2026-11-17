package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/customagents"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func setupIsolatedAppTestEnv(t *testing.T) (*config.ConfigStore, string, string) {
	t.Helper()
	homeDir := t.TempDir()
	globalCfgDir := filepath.Join(homeDir, "global-config")
	globalDataDir := filepath.Join(homeDir, "global-data")
	workDir := filepath.Join(homeDir, "work")
	dataDir := filepath.Join(workDir, ".prime")

	require.NoError(t, os.MkdirAll(globalCfgDir, 0o700))
	require.NoError(t, os.MkdirAll(globalDataDir, 0o700))
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("PRIME_GLOBAL_CONFIG", globalCfgDir)
	t.Setenv("PRIME_GLOBAL_DATA", globalDataDir)
	t.Setenv("PRIME_CACHE_DIR", filepath.Join(homeDir, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, ".cache"))

	cfgJSON := `{
  "options": {
    "disable_default_providers": true,
    "auto_lsp": false
  },
  "providers": {
    "mock-p": {
      "id": "mock-p",
      "name": "Mock Provider",
      "type": "openai-compat",
      "base_url": "http://127.0.0.1:0/v1",
      "api_key": "test-key",
      "auto_discover_models": false,
      "models": [
        {
          "id": "mock-m",
          "name": "Mock Model",
          "default_max_tokens": 2048
        }
      ]
    }
  },
  "models": {
    "large": {
      "provider": "mock-p",
      "model": "mock-m"
    },
    "small": {
      "provider": "mock-p",
      "model": "mock-m"
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(globalCfgDir, "prime.json"), []byte(cfgJSON), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(globalCfgDir, "SOUL.md"), []byte("Global config soul fallback"), 0o600))

	cfgStore, err := config.Init(workDir, dataDir, false)
	require.NoError(t, err)
	return cfgStore, dataDir, globalDataDir
}

func TestStandaloneAppNewInitializesAndShutsDownCustomAgents(t *testing.T) {
	cfgStore, dataDir, _ := setupIsolatedAppTestEnv(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	conn, err := db.Connect(ctx, dataDir)
	require.NoError(t, err)

	appInstance, err := New(ctx, conn, cfgStore, nil)
	require.NoError(t, err)
	require.NotNil(t, appInstance.Coordinator())

	svc := appInstance.CustomAgents()
	require.NotNil(t, svc)
	require.NotNil(t, svc.Store())

	agents, err := svc.Store().ListAgents(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(agents), 2)

	gm := svc.GlobalMemory()
	require.NotNil(t, gm)
	_, soul := gm.FormatFrozenSnapshot(ctx)
	require.Equal(t, "Global config soul fallback", soul)

	appInstance.Shutdown()

	_, postCloseErr := svc.Store().ListAgents(context.Background())
	require.Error(t, postCloseErr)
}

func TestAppPreservesInjectedCustomAgentsService(t *testing.T) {
	cfgStore, dataDir, globalDataDir := setupIsolatedAppTestEnv(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	conn, err := db.Connect(ctx, dataDir)
	require.NoError(t, err)

	injectedStore, err := customagents.OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = injectedStore.Close() })
	injectedSvc := customagents.NewService(injectedStore, nil, nil, nil)
	t.Cleanup(injectedSvc.Stop)

	appInstance, err := NewDeferred(ctx, conn, cfgStore, nil)
	require.NoError(t, err)
	t.Cleanup(appInstance.Shutdown)

	appInstance.SetCustomAgents(injectedSvc)
	require.NoError(t, appInstance.InitCoderAgent(ctx))
	require.Same(t, injectedSvc, appInstance.CustomAgents())

	_, statErr := os.Stat(filepath.Join(globalDataDir, "custom-agents", "custom_agents.db"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
