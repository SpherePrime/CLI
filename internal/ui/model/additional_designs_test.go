package model

import (
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestAdditionalDesignsSelectAndKeepComposerBelowChat(t *testing.T) {
	for _, design := range []string{"opencode", "paper", "blueprint", "ember"} {
		t.Run(design, func(t *testing.T) {
			require.True(t, appearance.ValidDesign(design))
			u, ws := newAppearanceFlowUI(t)
			u.width, u.height = 180, 50
			require.NoError(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "designs", Key: design}))
			require.Equal(t, design, u.com.Styles.Design)
			require.Equal(t, appearance.DesignSpec(design).Theme, ws.cfg.Options.TUI.Theme)
			require.GreaterOrEqual(t, u.layout.editor.Min.Y, u.layout.main.Max.Y)
			caption, _ := u.editorChromeParts(u.layout.editor.Dx())
			require.NotEmpty(t, caption)
		})
	}
}

func TestPaperDoesNotKeepSidebarFocusState(t *testing.T) {
	u, _ := newAppearanceFlowUI(t)
	u.width, u.height = 180, 50
	u.sidebarScrollable = true
	u.sidebarMaxOffsetVal = 20
	require.NoError(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "designs", Key: "paper"}))
	u.updateDesignSidebarScrollState()
	require.False(t, u.sidebarScrollable)
	require.Zero(t, u.sidebarMaxOffsetVal)
	require.True(t, u.designHidesSidebar())
}
