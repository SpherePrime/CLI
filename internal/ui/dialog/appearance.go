package dialog

import (
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

const ThemesID = "themes"
const DesignsID = "designs"

type ActionSelectAppearance struct{ Kind, Key string }
type Appearance struct {
	*Language
	kind string
}

func NewAppearance(com *common.Common, kind string) *Appearance {
	picker := &Appearance{Language: NewLanguage(com), kind: kind}
	cfg := com.Config()
	current := ""
	presets := appearance.Themes()
	picker.titleKey = "cmd.theme"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI != nil {
		current = cfg.Options.TUI.Theme
	}
	if kind == DesignsID {
		presets = appearance.Designs()
		picker.titleKey = "cmd.design"
		current = "classic"
		if cfg != nil && cfg.Options != nil && cfg.Options.TUI != nil {
			current = appearance.DesignKey(cfg.Options.TUI.Design)
		}
	} else {
		if current == "" || current == "auto" {
			current = ""
		} else {
			current = appearance.ThemeKey(current)
		}
		presets = append([]appearance.Preset{{Key: "", Title: com.L("theme.automatic"), Description: "Automatic / provider"}}, presets...)
	}
	items := make([]list.FilterableItem, 0, len(presets))
	selected := 0
	for index, preset := range presets {
		itemStyles := com.Styles
		if kind == ThemesID && preset.Key != "" {
			palette := styles.ThemeForName(preset.Key)
			itemStyles = &palette
		}
		item := &LanguageItem{Versioned: list.NewVersioned(), locale: preset.Key, title: preset.Title + " · " + preset.Description, isCurrent: preset.Key == current, t: itemStyles}
		if item.isCurrent {
			selected = index
		}
		items = append(items, item)
	}
	picker.list.SetItems(items...)
	picker.list.SetSelected(selected)
	picker.list.ScrollToSelected()
	return picker
}
func (p *Appearance) ID() string { return p.kind }
func (p *Appearance) HandleMsg(msg tea.Msg) Action {
	action := p.Language.HandleMsg(msg)
	if selected, ok := action.(ActionSelectLanguage); ok {
		return ActionSelectAppearance{Kind: p.kind, Key: selected.Locale}
	}
	return action
}
