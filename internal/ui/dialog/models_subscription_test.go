package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type subscriptionTestWorkspace struct {
	defaultModelWorkspace
	lang string
}

func (w *subscriptionTestWorkspace) Language() string {
	if w.lang != "" {
		return w.lang
	}
	return "en"
}

func (w *subscriptionTestWorkspace) SetConfigField(config.Scope, string, any) error {
	return nil
}

func TestModelsSubscriptionBadgesAndLocking(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("PRIME_GLOBAL_DATA", tmpDir)

	sty := providerTestStyles()
	prov := catwalk.Provider{
		ID:   primeai.Name,
		Name: primeai.DisplayName,
		Models: []catwalk.Model{
			{ID: primeai.AutomaticModelID, Name: primeai.AutomaticModelName},
			{ID: "gpt-5-pro", Name: "GPT-5 Pro", SupportsImages: true},
			{ID: "opus-ultra", Name: "Opus Ultra"},
		},
	}

	proItem := NewModelItem(sty, prov, prov.Models[1], ModelTypeLarge, false)
	proItem.SetAccess(primeai.ModelAccess{PlanLabel: "Pro", Locked: false})
	require.Equal(t, "[Pro]", proItem.SubscriptionBadge())
	require.False(t, proItem.IsLocked())
	require.Contains(t, proItem.Render(70), "[Pro]")

	ultraItem := NewModelItem(sty, prov, prov.Models[2], ModelTypeLarge, true)
	ultraItem.SetAccess(primeai.ModelAccess{PlanLabel: "Ultra", Locked: true, AllowedTariffNames: []string{"Ultra"}})
	require.Equal(t, "[🔒 Ultra]", ultraItem.SubscriptionBadge())
	require.True(t, ultraItem.IsLocked())
	rendered := ultraItem.Render(70)
	require.Contains(t, rendered, "[🔒 Ultra]")
	require.Contains(t, rendered, "PrimeAI")

	cfg := config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			primeai.Name: {
				ID:     primeai.Name,
				Name:   primeai.DisplayName,
				Type:   primeai.Name,
				Models: prov.Models,
			},
		}),
		RecentModels: map[config.SelectedModelType][]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				{Provider: primeai.Name, Model: "opus-ultra"},
			},
		},
		Options: &config.Options{},
	}

	ws := &subscriptionTestWorkspace{lang: "ru"}
	ws.cfg = cfg
	com := &common.Common{Workspace: ws, Styles: sty}
	m, err := NewModelsForAgent(com, false, "")
	require.NoError(t, err)

	m.providers = []catwalk.Provider{prov}
	m.accessLookup = func(modelID string) (primeai.ModelAccess, bool) {
		switch modelID {
		case "gpt-5-pro":
			return primeai.ModelAccess{PlanLabel: "Pro", Locked: false}, true
		case "opus-ultra":
			return primeai.ModelAccess{PlanLabel: "Ultra", Locked: true, AllowedTariffNames: []string{"Ultra"}}, true
		default:
			return primeai.ModelAccess{}, false
		}
	}
	require.NoError(t, m.setProviderItems())

	var recentUltraIdx = -1
	var providerProIdx = -1
	var providerUltraIdx = -1
	for i := 0; i < len(m.list.VisibleItems()); i++ {
		item, ok := m.list.ItemAt(i).(*ModelItem)
		if !ok {
			continue
		}
		if item.model.ID == "opus-ultra" && item.showProvider {
			recentUltraIdx = i
		}
		if item.model.ID == "opus-ultra" && !item.showProvider {
			providerUltraIdx = i
		}
		if item.model.ID == "gpt-5-pro" && !item.showProvider {
			providerProIdx = i
		}
	}

	require.NotEqual(t, -1, recentUltraIdx)
	require.NotEqual(t, -1, providerUltraIdx)
	require.NotEqual(t, -1, providerProIdx)

	recentUltra := m.list.ItemAt(recentUltraIdx).(*ModelItem)
	require.True(t, recentUltra.IsLocked())
	require.Equal(t, "[🔒 Ultra]", recentUltra.SubscriptionBadge())

	for _, idx := range []int{recentUltraIdx, providerUltraIdx} {
		act := m.activateModel(idx, false)
		cmdAct, ok := act.(ActionCmd)
		require.True(t, ok)
		require.NotNil(t, cmdAct.Cmd)
		msg, ok := cmdAct.Cmd().(util.InfoMsg)
		require.True(t, ok)
		require.Equal(t, util.InfoTypeWarn, msg.Type)
		require.Equal(t, "Доступно в подписке: Ultra", msg.Msg)
	}

	ws.lang = "en"
	actEn := m.activateModel(providerUltraIdx, false).(ActionCmd)
	msgEn := actEn.Cmd().(util.InfoMsg)
	require.Equal(t, "Available with Ultra subscription", msgEn.Msg)

	actPro := m.activateModel(providerProIdx, false)
	require.IsType(t, ActionSelectModel{}, actPro)
	require.Equal(t, "gpt-5-pro", actPro.(ActionSelectModel).Model.Model)
}
