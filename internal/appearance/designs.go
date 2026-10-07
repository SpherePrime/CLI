package appearance

type Design struct {
	Key, Title, Description, Theme, Sidebar              string
	SidebarWidth, InspectorWidth, HeaderHeight, MaxWidth int
	InputTop                                             bool
}

func DesignSpecs() []Design {
	return []Design{
		{Key: "classic", Title: "Classic", Description: "Prime / right sidebar", Theme: "default", Sidebar: "right", SidebarWidth: 32},
		{Key: "minimal", Title: "Minimal", Description: "Solarized / open canvas / inline prompt", Theme: "solarized", HeaderHeight: 2},
		{Key: "cards", Title: "Cards", Description: "Mocha / cards / composer above chat", Theme: "catppuccin", Sidebar: "right", SidebarWidth: 28, HeaderHeight: 3, InputTop: true},
		{Key: "focus", Title: "Focus", Description: "Rose / centered conversation / soft composer", Theme: "rose", HeaderHeight: 3, MaxWidth: 100},
		{Key: "dashboard", Title: "Dashboard", Description: "Nord / left workspace / framed panels", Theme: "nord", Sidebar: "left", SidebarWidth: 34, HeaderHeight: 3},
		{Key: "terminal", Title: "Terminal", Description: "Gruvbox / top command line / bottom status dock", Theme: "gruvbox", Sidebar: "bottom", HeaderHeight: 2, InputTop: true},
		{Key: "studio", Title: "Studio", Description: "Ocean / navigation / chat / right inspector", Theme: "ocean", Sidebar: "left", SidebarWidth: 24, InspectorWidth: 28, HeaderHeight: 2},
		{Key: "neon", Title: "Neon", Description: "Cyberpunk / control bar / bottom dock", Theme: "cyberpunk", Sidebar: "bottom", HeaderHeight: 4},
	}
}
func DesignSpec(name string) Design {
	for _, design := range DesignSpecs() {
		if design.Key == DesignKey(name) {
			return design
		}
	}
	return DesignSpecs()[0]
}
