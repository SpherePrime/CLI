package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestEditorPanelFitsWidthAndTracksCursor(t *testing.T) {
	for _, width := range []int{20, 80, 140} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			u := newSelectionTestUI()
			u.width = width
			u.setEditorPrompt(false)
			u.updateLayoutAndSize()
			u.textarea.SetValue("Привет\nworld")
			u.textarea.MoveToEnd()
			u.updateLayoutAndSize()

			view := ansi.Strip(u.renderEditorView(u.layout.editor.Dx()))
			lines := strings.Split(view, "\n")
			require.True(t, strings.HasPrefix(lines[0], "╭"), view)
			require.True(t, strings.HasSuffix(lines[len(lines)-1], "╯"), view)
			require.NotContains(t, view, ":::")
			require.Equal(t, u.layout.editor.Dy(), len(lines))
			for _, line := range lines {
				require.Equal(t, u.layout.editor.Dx(), lipgloss.Width(line))
			}
			cursor := u.completionsPosition()
			row := lines[cursor.Y-u.layout.editor.Min.Y]
			prefix := row[:strings.Index(row, "world")+len("world")]
			require.Equal(t, lipgloss.Width(prefix), cursor.X-u.layout.editor.Min.X)
		})
	}
}

func TestEditorPanelAttachmentsStayAboveFrame(t *testing.T) {
	u := newSelectionTestUI()
	u.setEditorPrompt(false)
	u.attachments.Update(message.Attachment{FileName: "notes.txt", MimeType: "text/plain"})
	u.textarea.SetValue("hello")
	u.textarea.MoveToEnd()
	u.updateLayoutAndSize()
	lines := strings.Split(ansi.Strip(u.renderEditorView(u.layout.editor.Dx())), "\n")
	require.Contains(t, lines[0], "notes.txt")
	require.True(t, strings.HasPrefix(lines[1], "╭"))
	require.Equal(t, u.layout.editor.Dy(), len(lines))
	require.Equal(t, u.layout.editor.Min.Y+2, u.completionsPosition().Y)
}

func TestEditorPanelWrappedTextAndMouse(t *testing.T) {
	u := newSelectionTestUI()
	u.width = 30
	u.setEditorPrompt(false)
	u.updateLayoutAndSize()
	u.textarea.SetValue("abcdefghijklmnopqrstuvwxyz")
	u.textarea.MoveToEnd()
	u.updateLayoutAndSize()
	lines := strings.Split(ansi.Strip(u.renderEditorView(u.layout.editor.Dx())), "\n")
	cursor := u.completionsPosition()
	row := lines[cursor.Y-u.layout.editor.Min.Y]
	require.Contains(t, row, "z")
	prefix := row[:strings.Index(row, "z")+1]
	require.Equal(t, lipgloss.Width(prefix), cursor.X-u.layout.editor.Min.X)
	_, _ = u.Update(tea.MouseClickMsg(tea.Mouse{
		X: u.layout.editor.Min.X, Y: u.layout.editor.Min.Y + 1, Button: uv.MouseLeft,
	}))
	require.False(t, u.textareaMouseSelecting)
}

func TestEditorPanelKeepsTextVisibleInTinyTerminal(t *testing.T) {
	for width := 7; width <= 10; width++ {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			u := newSelectionTestUI()
			u.width = width
			u.setEditorPrompt(false)
			u.textarea.SetValue("a")
			u.textarea.CursorStart()
			u.updateLayoutAndSize()
			view := ansi.Strip(u.renderEditorView(u.layout.editor.Dx()))
			require.Contains(t, view, "a")
			require.True(t, u.completionsPosition().In(u.layout.editor))
			for _, line := range strings.Split(view, "\n") {
				require.LessOrEqual(t, lipgloss.Width(line), u.layout.editor.Dx())
			}
		})
	}
}
