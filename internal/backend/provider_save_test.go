package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSaveProviderRefreshesAllOpenWorkspaces(t *testing.T) {
	b, first, _ := newPublishingWorkspace(t)
	second, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), uuid.New().String()))
	require.NoError(t, err)
	b.SetCreateGrace(2 * time.Second)
	key := "fixture-key"
	request := proto.ProviderSaveRequest{Scope: config.ScopeGlobal, APIKey: &key, Provider: config.ProviderEdit{
		ID: "provider-save-fixture", Name: "Fixture", Type: "openai-compat", BaseURL: "http://localhost:11434/v1", DiscoverModels: false,
		Models: []map[string]any{{"id": "partial", "name": "Partial"}, {"id": "second", "name": "Second"}},
	}}
	require.NoError(t, b.SaveProvider(first.ID, request))
	provider, exists := second.Cfg.Config().Providers.Get(request.Provider.ID)
	require.True(t, exists)
	require.Len(t, provider.Models, 2)
	require.NoError(t, first.Cfg.SetSessionModel("first-chat", &config.SelectedModel{Provider: request.Provider.ID, Model: "partial"}))
	require.NoError(t, second.Cfg.SetSessionModel("second-chat", &config.SelectedModel{Provider: request.Provider.ID, Model: "second"}))
	request.OriginalID = request.Provider.ID
	request.Provider.ID = "provider-save-renamed"
	require.NoError(t, b.SaveProvider(second.ID, request))
	_, exists = first.Cfg.Config().Providers.Get(request.OriginalID)
	require.False(t, exists)
	provider, exists = first.Cfg.Config().Providers.Get(request.Provider.ID)
	require.True(t, exists)
	require.Len(t, provider.Models, 2)
	for _, opened := range []*Workspace{first, second} {
		for _, selected := range opened.Cfg.Config().SessionModels {
			require.Equal(t, request.Provider.ID, selected.Provider)
		}
		data, err := os.ReadFile(filepath.Join(opened.Cfg.Config().Options.DataDirectory, "models.json"))
		require.NoError(t, err)
		var saved struct {
			SessionModels map[string]config.SelectedModel `json:"session_models"`
		}
		require.NoError(t, json.Unmarshal(data, &saved))
		require.NotEmpty(t, saved.SessionModels)
		for _, selected := range saved.SessionModels {
			require.Equal(t, request.Provider.ID, selected.Provider)
		}
	}
}
