package model

import (
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"strings"
	"testing"
)

func TestDesignEditorsHaveDistinctChromeAndAccurateCursor(t *testing.T) {
	cases := []struct{ design, caption string }{{"minimal", ">"}, {"cards", "NEW MESSAGE"}, {"focus", "MESSAGE"}, {"dashboard", "COMPOSE"}, {"terminal", "$"}, {"studio", "COMPOSE"}, {"neon", "TRANSMIT"}, {"opencode", "COMPOSE"}, {"paper", "COMPOSE"}, {"blueprint", "COMPOSE"}, {"ember", "COMPOSE"}}
	for _, tc := range cases {
		t.Run(tc.design, func(t *testing.T) {
			u := newSelectionTestUI()
			u.width, u.height = 180, 50
			*u.com.Styles = styles.ResolveTheme("nord", "", tc.design)
			u.textarea.SetStyles(u.com.Styles.Editor.Textarea)
			u.setEditorPrompt(false)
			u.textarea.SetValue("hello\nworld")
			u.textarea.MoveToEnd()
			u.updateLayoutAndSize()
			view := ansi.Strip(u.renderEditorView(u.layout.editor.Dx()))
			require.Contains(t, view, tc.caption)
			lines := strings.Split(view, "\n")
			require.Equal(t, len(lines), u.layout.editor.Dy())
			cursor := u.completionsPosition()
			require.True(t, cursor.In(u.layout.editor))
			row := lines[cursor.Y-u.layout.editor.Min.Y]
			require.Contains(t, row, "world")
			prefix := row[:strings.Index(row, "world")+len("world")]
			require.Equal(t, lipgloss.Width(prefix), cursor.X-u.layout.editor.Min.X)
		})
	}
}
