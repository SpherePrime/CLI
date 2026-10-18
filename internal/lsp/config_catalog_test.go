package lsp

import (
	"context"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestTrackConfiguredReadsUpdatedCatalog(t *testing.T) {
	cfg := &config.Config{LSP: config.LSPs{}}
	manager := NewManager(config.NewTestStore(cfg))
	var names []string
	var mu sync.Mutex
	manager.SetCallback(func(name string, _ *Client) { mu.Lock(); defer mu.Unlock(); names = append(names, name) })
	cfg.LSP["desktop-fixture"] = config.LSPConfig{Command: "fixture", FileTypes: []string{"fixture"}}
	manager.TrackConfigured(context.Background())
	require.Contains(t, names, "desktop-fixture")
	names = nil
	cfg.LSP["desktop-fixture"] = config.LSPConfig{Disabled: true}
	manager.TrackConfigured(context.Background())
	require.NotContains(t, names, "desktop-fixture")
}

func TestConfiguredServersReadsEditedFields(t *testing.T) {
	cfg := &config.Config{LSP: config.LSPs{"fixture": {Command: "before"}}}
	manager := NewManager(config.NewTestStore(cfg))
	require.Equal(t, "before", manager.configuredServers()["fixture"].Command)
	cfg.LSP["fixture"] = config.LSPConfig{Command: "after", Args: []string{"--stdio"}, Env: map[string]string{"TEST": "value"}, FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}, InitOptions: map[string]any{"enabled": true}, Options: map[string]any{"mode": "test"}}
	server := manager.configuredServers()["fixture"]
	require.Equal(t, "after", server.Command)
	require.Equal(t, []string{"--stdio"}, server.Args)
	require.Equal(t, map[string]string{"TEST": "value"}, server.Environment)
	require.Equal(t, []string{"go"}, server.FileTypes)
	require.Equal(t, []string{"go.mod"}, server.RootMarkers)
	require.Equal(t, map[string]any{"enabled": true}, server.InitOptions)
	require.Equal(t, map[string]any{"mode": "test"}, server.Settings)
	delete(cfg.LSP, "fixture")
	require.NotContains(t, manager.configuredServers(), "fixture")
}
