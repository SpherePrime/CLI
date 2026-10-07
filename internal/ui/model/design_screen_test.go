package model

import (
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/ui/chat"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesignScreensKeepEditorAndMessagesVisible(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "focus", "dashboard", "terminal", "studio", "neon"} {
		t.Run(design, func(t *testing.T) {
			u, _ := newAppearanceFlowUI(t)
			u.width, u.height = 160, 44
			u.status = NewStatus(u.com, u)
			u.session = &session.Session{ID: "preview", Title: "CLI interface designs"}
			*u.com.Styles = styles.ResolveTheme("", "acme", design)
			u.setEditorPrompt(false)
			u.textarea.SetValue("Improve the command palette and verify the result.")
			user := &message.Message{ID: "user-preview", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Add interface designs and a theme picker."}}}
			reply := &message.Message{ID: "reply-preview", Role: message.Assistant, Model: "fast", Provider: "acme", Parts: []message.ContentPart{message.TextContent{Text: "The command palette now contains interface designs and color themes. Selection is saved between sessions."}, message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()}}}
			call := message.ToolCall{ID: "grep-preview", Name: "grep", Finished: true, Input: `{"pattern":"theme", "path":"internal/ui"}`}
			result := &message.ToolResult{ToolCallID: call.ID, Content: "internal/ui/styles/theme_catalog.go:14: ThemeForName"}
			u.chat.SetMessages(chat.NewUserMessageItem(u.com.Styles, user, nil), chat.NewGrepToolMessageItem(u.com.Styles, call, result, false), chat.NewAssistantMessageItem(u.com.Styles, reply), chat.NewAssistantInfoItem(u.com.Styles, reply, u.com.Config(), time.Now().Add(-3*time.Second)), chat.NewAssistantMessageItem(u.com.Styles, &message.Message{ID: "working-preview", Role: message.Assistant}))
			u.updateLayoutAndSize()
			scr := uv.NewScreenBuffer(u.width, u.height)
			u.Draw(scr, uv.Rect(0, 0, u.width, u.height))
			rendered := scr.Render()
			text := ansi.Strip(rendered)
			require.Contains(t, text, "Improve the command palette")
			require.Contains(t, text, "ThemeForName")
			require.Contains(t, text, "Add interface designs")
			for _, line := range strings.Split(text, "\n") {
				require.LessOrEqual(t, ansi.StringWidth(line), u.width)
			}
			if directory := os.Getenv("PRIME_DESIGN_PREVIEW_DIR"); directory != "" {
				require.NoError(t, os.MkdirAll(directory, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(directory, design+".ansi"), []byte(rendered), 0644))
			}
		})
	}
}
