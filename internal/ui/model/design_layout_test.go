package model

import (
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestDesignChangesLayout(t *testing.T) {
	for _, design := range []string{"focus", "minimal", "dashboard"} {
		t.Run(design, func(t *testing.T) {
			u := layoutTestUI(false, "hello")
			u.state = uiChat
			*u.com.Styles = styles.ApplyDesign(*u.com.Styles, design)
			l := u.generateLayout(180, 50)
			switch design {
			case "focus", "minimal":
				require.Zero(t, l.sidebar.Dx())
				require.Positive(t, l.header.Dy())
			case "dashboard":
				require.Less(t, l.sidebar.Max.X, l.main.Min.X)
			}
		})
	}
}

func TestDesignEditorWidthMatchesActualLayout(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "focus", "dashboard", "terminal", "studio", "neon", "opencode", "paper", "blueprint", "ember"} {
		for _, width := range []int{20, 80, 140, 180} {
			u := layoutTestUI(false, "hello")
			u.width = width
			*u.com.Styles = styles.ApplyDesign(*u.com.Styles, design)
			l := u.generateLayout(width, u.height)
			require.Equal(t, l.editor.Dx(), u.editorContentWidth(), "%s at %d", design, width)
		}
	}
}

func TestCompleteDesignPresetsChangePaletteAndPlacement(t *testing.T) {
	cases := []struct{ design, theme string }{{"minimal", "solarized"}, {"cards", "catppuccin"}, {"focus", "rose"}, {"dashboard", "nord"}, {"terminal", "gruvbox"}, {"studio", "ocean"}, {"neon", "cyberpunk"}}
	for _, tc := range cases {
		t.Run(tc.design, func(t *testing.T) {
			u, ws := newAppearanceFlowUI(t)
			u.width, u.height = 180, 50
			require.NoError(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "designs", Key: tc.design}))
			require.Equal(t, tc.theme, ws.cfg.Options.TUI.Theme)
			l := u.generateLayout(180, 50)
			require.Positive(t, l.header.Dy())
			switch tc.design {
			case "cards", "terminal":
				require.GreaterOrEqual(t, l.editor.Min.Y, l.main.Max.Y)
			case "neon":
				require.GreaterOrEqual(t, l.sidebar.Min.Y, l.main.Max.Y)
			}
		})
	}
}

func TestDesignLayoutFitsSmallWindows(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "focus", "dashboard", "terminal", "studio", "neon", "opencode", "paper", "blueprint", "ember"} {
		for _, width := range []int{1, 2, 20, 80, 120, 160} {
			for _, height := range []int{1, 2, 6, 12, 26, 44} {
				u := layoutTestUI(false, "hello")
				*u.com.Styles = styles.ApplyDesign(*u.com.Styles, design)
				for name, rect := range layoutAreas(u.generateLayout(width, height)) {
					require.GreaterOrEqual(t, rect.Dx(), 0, "%s %s", design, name)
					require.GreaterOrEqual(t, rect.Dy(), 0, "%s %s", design, name)
					require.GreaterOrEqual(t, rect.Min.X, 0)
					require.GreaterOrEqual(t, rect.Min.Y, 0)
					require.LessOrEqual(t, rect.Max.X, width)
					require.LessOrEqual(t, rect.Max.Y, height)
				}
			}
		}
	}
}

func TestDesignSelectionRestoresFocusWhenSidebarDisappears(t *testing.T) {
	for _, design := range []string{"focus", "minimal"} {
		u, _ := newAppearanceFlowUI(t)
		u.width, u.height = 180, 50
		u.focus = uiFocusSidebar
		require.NoError(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "designs", Key: design}))
		require.Equal(t, uiFocusMain, u.focus)
	}
}
