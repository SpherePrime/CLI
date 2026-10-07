package model

import (
	"fmt"
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/ui/styles"
)

func configuredThemeName(cfg *config.Config) string {
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil {
		return ""
	}
	return cfg.Options.TUI.Theme
}
func configuredDesignName(cfg *config.Config) string {
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil {
		return "classic"
	}
	return appearance.DesignKey(cfg.Options.TUI.Design)
}
func (m *UI) selectAppearance(action dialog.ActionSelectAppearance) error {
	cfg := m.com.Config()
	if cfg == nil {
		return fmt.Errorf("configuration not found")
	}
	key, value := "options.tui.theme", action.Key
	switch action.Kind {
	case dialog.ThemesID:
		if !appearance.ValidTheme(value) {
			return fmt.Errorf("unknown theme %q", value)
		}
		if value == "auto" {
			if configuredDesignName(cfg) == "classic" {
				value = ""
			}
		} else if value != "" {
			value = appearance.ThemeKey(value)
		}
	case dialog.DesignsID:
		key = "options.tui.design"
		if !appearance.ValidDesign(value) {
			return fmt.Errorf("unknown design %q", value)
		}
		value = appearance.DesignKey(value)
	default:
		return fmt.Errorf("unknown appearance picker %q", action.Kind)
	}
	if action.Kind == dialog.ThemesID && value == "" && configuredDesignName(cfg) != "classic" {
		value = "auto"
	}
	if err := m.persistAppearance(action.Kind, key, value); err != nil {
		return err
	}
	if action.Kind == dialog.DesignsID {
		theme := appearance.DesignSpec(value).Theme
		if err := m.persistAppearance(dialog.ThemesID, "options.tui.theme", theme); err != nil {
			return err
		}
	}
	cfg = m.com.Config()
	provider := cfg.Models[config.SelectedModelTypeLarge].Provider
	m.themeKey = styles.ResolveDesignThemeKey(cfg.Options.TUI.Theme, provider, cfg.Options.TUI.Design)
	m.applyTheme(styles.ResolveTheme(cfg.Options.TUI.Theme, provider, cfg.Options.TUI.Design))
	m.dialog.CloseDialog(action.Kind)
	m.dialog.CloseDialog(dialog.CommandsID)
	m.updateLayoutAndSize()
	return nil
}

func (m *UI) designHidesSidebar() bool {
	return m.com.Styles.Design == "focus" || m.com.Styles.Design == "minimal"
}
func (m *UI) designSidebarWidth() int {
	switch m.com.Styles.Design {
	case "dashboard":
		return 38
	case "cards":
		return 28
	case "terminal":
		return 26
	default:
		return 32
	}
}

func appearanceMatches(cfg *config.Config, kind, value string) bool {
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil {
		return false
	}
	if kind == dialog.DesignsID {
		return configuredDesignName(cfg) == value
	}
	return configuredThemeName(cfg) == value
}

func (m *UI) persistAppearance(kind, key, value string) error {
	if err := m.com.Workspace.SetConfigField(config.ScopeGlobal, key, value); err != nil {
		return err
	}
	if !appearanceMatches(m.com.Config(), kind, value) {
		if err := m.com.Workspace.SetConfigField(config.ScopeWorkspace, key, value); err != nil {
			return err
		}
	}
	if !appearanceMatches(m.com.Config(), kind, value) {
		return fmt.Errorf("appearance setting is overridden by project configuration")
	}
	return nil
}
