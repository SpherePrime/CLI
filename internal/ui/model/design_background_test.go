package model

import (
	"github.com/SpherePrime/CLI/internal/ui/styles"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestDesignDrawKeepsBackgroundOnEveryCell(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "focus", "dashboard", "terminal", "studio", "neon"} {
		t.Run(design, func(t *testing.T) {
			u, _ := newAppearanceFlowUI(t)
			u.width, u.height = 160, 44
			u.status = NewStatus(u.com, u)
			for _, theme := range []string{"rose", "nord", "solarized"} {
				u.applyTheme(styles.ResolveTheme(theme, "acme", design))
				u.textarea.SetValue("hello")
				u.updateLayoutAndSize()
				buffer := uv.NewScreenBuffer(u.width, u.height)
				u.Draw(buffer, buffer.Bounds())
				for y := 0; y < u.height; y++ {
					for x := 0; x < u.width; x++ {
						require.NotNil(t, buffer.CellAt(x, y).Style.Bg, "%s %s at %d,%d", design, theme, x, y)
					}
				}
				wantR, wantG, wantB, wantA := u.com.Styles.Background.RGBA()
				gotR, gotG, gotB, gotA := buffer.CellAt(0, 0).Style.Bg.RGBA()
				require.Equal(t, []uint32{wantR, wantG, wantB, wantA}, []uint32{gotR, gotG, gotB, gotA})
			}
		})
	}
}

func TestFlatDesignEditorAndHeaderUseCanvasBackground(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "terminal"} {
		theme := styles.ResolveTheme("solarized", "", design)
		require.Equal(t, theme.Background, theme.Canvas.GetBackground(), design)
		require.Equal(t, theme.Background, theme.Editor.PanelFocused.GetBackground(), design)
		if design != "classic" {
			require.Equal(t, theme.Background, theme.DesignHeader.GetBackground(), design)
		}
	}
}

func TestBackgroundScreenPreservesExplicitColorsAndWideCells(t *testing.T) {
	base := styles.ResolveTheme("nord", "", "classic").Background
	buffer := uv.NewScreenBuffer(6, 1)
	screen := backgroundScreen{Screen: buffer, background: base}
	uv.NewStyledString("界\x1b[48;2;1;2;3mX\x1b[m Y").Draw(screen, screen.Bounds())
	require.Equal(t, "界", buffer.CellAt(0, 0).Content)
	require.Equal(t, 2, buffer.CellAt(0, 0).Width)
	r, g, b, _ := buffer.CellAt(2, 0).Style.Bg.RGBA()
	require.Equal(t, []uint32{257, 514, 771}, []uint32{r, g, b})
	require.Equal(t, base, buffer.CellAt(4, 0).Style.Bg)
}

func TestTransparentDesignDoesNotForceCanvasBackground(t *testing.T) {
	u, _ := newAppearanceFlowUI(t)
	u.width, u.height = 160, 44
	u.status = NewStatus(u.com, u)
	u.isTransparent = true
	buffer := uv.NewScreenBuffer(u.width, u.height)
	u.Draw(buffer, buffer.Bounds())
	require.Nil(t, buffer.CellAt(0, 0).Style.Bg)
}
