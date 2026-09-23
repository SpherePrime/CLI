package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func providerTestStyles() *styles.Styles {
	s := styles.ColorTonePantera()
	return &s
}

func newProviderTestDialog(t *testing.T, models []catwalk.Model) (*ProviderSettings, *providerSettingsWorkspace) {
	t.Helper()

	ws := &providerSettingsWorkspace{cfg: config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}}
	ws.cfg.Providers.Set("osnova", config.ProviderConfig{
		ID:             "osnova",
		UserConfigured: true,
		Models:         models,
	})
	// A built-in catalog entry with no config of its own must not be listed.
	ws.cfg.Providers.Set("anthropic", config.ProviderConfig{ID: "anthropic"})

	return NewProviderSettings(&common.Common{
		Workspace: ws,
		Styles:    providerTestStyles(),
	}), ws
}

func TestConfiguredProviderItemsSkipsBuiltinCatalog(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	cfg.Providers.Set("osnova", config.ProviderConfig{
		ID:             "osnova",
		Name:           "Osnova",
		UserConfigured: true,
		Models:         []catwalk.Model{{ID: "a"}},
	})
	cfg.Providers.Set("anthropic", config.ProviderConfig{ID: "anthropic"})

	items := configuredProviderItems(providerTestStyles(), &cfg)
	require.Len(t, items, 1)

	item, ok := items[0].(*providerListItem)
	require.True(t, ok)
	// The row identifier must be the config key, since that is what the
	// dialog writes back to providers.<id>.models.
	require.Equal(t, "osnova", item.ID())
	require.Contains(t, item.Render(40), "osnova")
}

func TestProviderSettingsAddModel(t *testing.T) {
	t.Parallel()

	d, ws := newProviderTestDialog(t, []catwalk.Model{{ID: "existing"}})
	require.True(t, d.openProvider("osnova"))

	// An empty model ID must not be stored.
	d.input.SetValue("  ")
	require.NotNil(t, d.commitModelID())
	require.Len(t, d.models, 1)

	// A duplicate must be rejected.
	d.input.SetValue("existing")
	require.NotNil(t, d.commitModelID())
	require.Len(t, d.models, 1)

	// A new ID moves to the context field, then stores the model.
	d.input.SetValue("reasoning-9")
	action := d.commitModelID()
	require.Nil(t, action)
	require.Equal(t, providerSettingsStateModelContext, d.state)

	d.input.SetValue("200000")
	require.IsType(t, ActionProviderSettingsChanged{}, d.commitModelContext())
	require.Len(t, d.models, 2)
	require.Equal(t, "reasoning-9", d.models[1].ID)
	require.EqualValues(t, 200000, d.models[1].ContextWindow)
	require.Equal(t, []string{"providers.osnova.models"}, ws.writtenKeys)
}

func TestProviderSettingsAddModelRejectsBadContext(t *testing.T) {
	t.Parallel()

	d, ws := newProviderTestDialog(t, nil)
	require.True(t, d.openProvider("osnova"))

	d.startAddingModel()
	d.input.SetValue("new-model")
	require.Nil(t, d.commitModelID())

	d.input.SetValue("not-a-number")
	require.NotNil(t, d.commitModelContext())
	require.Empty(t, d.models)
	require.Empty(t, ws.writtenKeys)

	// An empty context window is allowed: the provider does not report one.
	d.input.SetValue("")
	require.IsType(t, ActionProviderSettingsChanged{}, d.commitModelContext())
	require.Len(t, d.models, 1)
	require.EqualValues(t, 0, d.models[0].ContextWindow)
}

func TestProviderSettingsRemoveModel(t *testing.T) {
	t.Parallel()

	d, ws := newProviderTestDialog(t, []catwalk.Model{{ID: "keep"}, {ID: "drop"}})
	require.True(t, d.openProvider("osnova"))

	d.list.SetSelected(1)
	require.IsType(t, ActionProviderSettingsChanged{}, d.removeSelectedModel())
	require.Len(t, d.models, 1)
	require.Equal(t, "keep", d.models[0].ID)
	require.Equal(t, []string{"providers.osnova.models"}, ws.writtenKeys)
}

func TestProviderSettingsEscapeStepsBack(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, []catwalk.Model{{ID: "a"}})

	d.startAddingModel()
	require.Nil(t, d.handleClose())
	require.Equal(t, providerSettingsStateModels, d.state)

	require.Nil(t, d.handleClose())
	require.Equal(t, providerSettingsStateProviders, d.state)

	require.IsType(t, ActionClose{}, d.handleClose())
}

// providerSettingsWorkspace is a minimal workspace that records config writes.
type providerSettingsWorkspace struct {
	workspace.Workspace
	cfg         config.Config
	writtenKeys []string
}

func (w *providerSettingsWorkspace) Config() *config.Config {
	return &w.cfg
}

func (w *providerSettingsWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	w.writtenKeys = append(w.writtenKeys, key)
	return nil
}
