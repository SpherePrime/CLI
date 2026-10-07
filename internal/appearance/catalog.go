package appearance

import "strings"

type Preset struct{ Key, Title, Description string }

func Themes() []Preset {
	return []Preset{
		{"default", "Prime", "Violet / jade"}, {"hyper", "Obsidian", "Magenta / midnight"},
		{"charcoal", "OLED", "Black / cyan"}, {"rose", "Rose", "Rose / plum"},
		{"stardust", "Stardust", "Amber / gold"}, {"forest", "Forest", "Pine / mint"},
		{"dracula", "Dracula", "Purple / pink"}, {"nord", "Nord", "Arctic blue"},
		{"tokyo", "Tokyo Night", "Neon / indigo"}, {"catppuccin", "Catppuccin", "Pastel / mocha"},
		{"gruvbox", "Gruvbox", "Earth / copper"}, {"solarized", "Solarized", "Teal / sand"},
		{"ocean", "Ocean", "Deep blue / aqua"}, {"cyberpunk", "Cyberpunk", "Electric pink / yellow"},
		{"sunset", "Sunset", "Coral / warm purple"},
	}
}
func Designs() []Preset {
	presets := make([]Preset, 0, len(DesignSpecs()))
	for _, design := range DesignSpecs() {
		presets = append(presets, Preset{design.Key, design.Title, design.Description})
	}
	return presets
}

func ThemeKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "oled":
		name = "charcoal"
	case "panthera":
		name = "default"
	}
	for _, preset := range Themes() {
		if preset.Key == name {
			return name
		}
	}
	return "default"
}
func ValidTheme(name string) bool {
	return name == "auto" || name == "" || name == "oled" || name == "panthera" || strings.ToLower(strings.TrimSpace(name)) == ThemeKey(name)
}
func DesignKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, preset := range Designs() {
		if preset.Key == name {
			return name
		}
	}
	return "classic"
}
func ValidDesign(name string) bool {
	return name == "" || name == "classic" || strings.ToLower(strings.TrimSpace(name)) == DesignKey(name)
}
