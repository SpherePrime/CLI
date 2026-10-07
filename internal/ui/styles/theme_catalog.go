package styles

import "github.com/SpherePrime/CLI/internal/appearance"

func ThemeKey(name string) string { return appearance.ThemeKey(name) }
func ThemeForName(name string) Styles {
	switch ThemeKey(name) {
	case "charcoal":
		return CharcoalOLED()
	case "rose":
		return RosePine()
	case "stardust":
		return Stardust()
	case "forest":
		return Forest()
	case "hyper":
		return HyperprimeObsidiana()
	case "default":
		return ColorTonePantera()
	default:
		return namedPaletteTheme(ThemeKey(name))
	}
}
func AllThemeNames() []string {
	names := make([]string, 0, len(appearance.Themes()))
	for _, theme := range appearance.Themes() {
		names = append(names, theme.Key)
	}
	return names
}
func ThemeNamesForProvider() []string { return AllThemeNames() }
func ResolveThemeKey(name, provider string) string {
	if name == "" || name == "auto" {
		return ThemeKeyForProvider(provider)
	}
	return ThemeKey(name)
}
func ResolveTheme(name, provider, design string) Styles {
	return ApplyDesign(ThemeForName(ResolveDesignThemeKey(name, provider, design)), design)
}

func ResolveDesignThemeKey(name, provider, design string) string {
	if name == "" && appearance.DesignKey(design) != "classic" {
		return appearance.DesignSpec(design).Theme
	}
	return ResolveThemeKey(name, provider)
}
