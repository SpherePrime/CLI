package backend

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSmartToolsPersistsAcrossDesktopBackendRestart(t *testing.T) {
	for _, scope := range []config.Scope{config.ScopeGlobal, config.ScopeWorkspace} {
		t.Run(string(rune('0'+scope)), func(t *testing.T) {
			xdgIsolated(t)
			globalDir := t.TempDir()
			t.Setenv("PRIME_GLOBAL_CONFIG", globalDir)
			t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())
			require.NoError(t, os.WriteFile(filepath.Join(globalDir, "options.json"), []byte(`{"options":{"disable_default_providers":true,"disable_provider_auto_update":true,"disable_metrics":true}}`), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(globalDir, "providers.json"), []byte(`{"providers":{"fixture":{"id":"fixture","type":"openai-compat","base_url":"http://127.0.0.1:9/v1","api_key":"fixture-key","discover_models":false,"models":[{"id":"fixture","name":"Fixture","context_window":8192,"default_max_tokens":128}]}}}`), 0o600))
			project := t.TempDir()
			dataDir := filepath.Join(project, ".prime")
			open := func() (*Backend, *Workspace) {
				b := New(context.Background(), nil, func() {})
				t.Cleanup(b.CloseInProcess)
				ws, _, err := b.CreateWorkspace(protoWS(project, dataDir, uuid.New().String()))
				require.NoError(t, err)
				return b, ws
			}
			b, ws := open()
			for _, enabled := range []bool{true, false, true} {
				require.NoError(t, b.SetConfigField(ws.ID, scope, "options.smart_tools", enabled))
				require.Equal(t, enabled, ws.Cfg.Config().Options.SmartTools)
				dir := globalDir
				if scope == config.ScopeWorkspace {
					dir = dataDir
				}
				contents, err := os.ReadFile(filepath.Join(dir, "options.json"))
				require.NoError(t, err)
				var saved struct {
					Options struct {
						SmartTools *bool `json:"smart_tools"`
					} `json:"options"`
				}
				require.NoError(t, json.Unmarshal(contents, &saved))
				require.NotNil(t, saved.Options.SmartTools)
				require.Equal(t, enabled, *saved.Options.SmartTools)
				b.CloseInProcess()
				b, ws = open()
				require.Equal(t, enabled, ws.Cfg.Config().Options.SmartTools)
			}
		})
	}
}
