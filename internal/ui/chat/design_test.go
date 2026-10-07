package chat

import (
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"image"
	"strings"
	"testing"
)

func TestCardsFrameMessagesWithoutOverflow(t *testing.T) {
	for _, width := range []int{1, 8, 20, 80} {
		sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
		user := newTestUserItem(t, "Hello world, this message should wrap cleanly.")
		user.sty = &sty
		assistant := NewAssistantMessageItem(&sty, finishedAssistantMessage("reply", "A useful reply.")).(*AssistantMessageItem)
		for _, item := range []MessageItem{user, assistant} {
			rendered := ansi.Strip(item.Render(width))
			if width >= 8 {
				require.Contains(t, rendered, "\u256d")
			}
			for _, line := range strings.Split(rendered, "\n") {
				require.LessOrEqual(t, ansi.StringWidth(line), width)
			}
		}
	}
}

func TestCardsFrameToolCalls(t *testing.T) {
	sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
	call := message.ToolCall{ID: "grep-1", Name: "grep", Finished: true, Input: `{"pattern":"hello"}`}
	result := &message.ToolResult{ToolCallID: call.ID, Content: "internal/main.go:12: hello"}
	item := NewGrepToolMessageItem(&sty, call, result, false)
	rendered := ansi.Strip(item.Render(80))
	require.Contains(t, rendered, "\u256d")
	require.Contains(t, rendered, "TOOL")
	require.Contains(t, rendered, "internal/main.go")
}

func TestDesignFullFrameAndCopy(t *testing.T) {
	for _, design := range []string{"cards", "dashboard", "studio", "neon", "blueprint"} {
		t.Run(design, func(t *testing.T) {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), design)
			sty.Design = design
			item := newTestUserItem(t, "Hello world")
			item.sty = &sty
			rendered := strings.Split(ansi.Strip(item.Render(30)), "\n")
			raw := strings.Split(ansi.Strip(item.RawRender(30)), "\n")
			require.Contains(t, rendered[0], "YOU")
			require.True(t, strings.ContainsAny(rendered[0], "─═━"))
			require.True(t, strings.ContainsAny(rendered[len(rendered)-1], "─═━"))
			require.Equal(t, len(rendered), len(raw))
			require.Empty(t, strings.TrimSpace(raw[0]))
			require.Empty(t, strings.TrimSpace(raw[len(raw)-1]))
			require.Equal(t, "Hello world", strings.TrimSpace(strings.Join(raw, "\n")))
			item.SetHighlight(1, 2, 1, 7)
			startLine, startCol, endLine, endCol := item.Highlight()
			selected := list.HighlightContent(item.RawRender(30), image.Rect(0, 0, 30, len(raw)), startLine, startCol, endLine, endCol)
			require.Equal(t, "Hello", strings.TrimSpace(selected))
		})
	}
}

func TestAdditionalDesignPrefixesAndCopy(t *testing.T) {
	for _, test := range []struct{ design, prefix string }{{"opencode", "┃ "}, {"paper", "› "}, {"ember", "▌ "}} {
		t.Run(test.design, func(t *testing.T) {
			sty := styles.ColorTonePantera()
			sty.Design = test.design
			item := newTestUserItem(t, "Hello world")
			item.sty = &sty
			require.True(t, strings.HasPrefix(ansi.Strip(item.Render(30)), test.prefix))
			item.SetHighlight(0, 2, 0, 7)
			startLine, startCol, endLine, endCol := item.Highlight()
			selected := list.HighlightContent(item.RawRender(30), image.Rect(0, 0, 30, 1), startLine, startCol, endLine, endCol)
			require.Equal(t, "Hello", strings.TrimSpace(selected))
		})
	}
}

func TestDesignSwitchChangesCachedMessages(t *testing.T) {
	sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
	item := newTestUserItem(t, "Hello world")
	item.sty = &sty
	require.Contains(t, ansi.Strip(item.Render(30)), "YOU")
	sty.Design = "terminal"
	rendered := ansi.Strip(item.Render(30))
	require.NotContains(t, rendered, "YOU")
	require.True(t, strings.HasPrefix(rendered, "> "))
	sty.Design = "minimal"
	rendered = ansi.Strip(item.Render(30))
	require.True(t, strings.HasPrefix(rendered, "  "))
	sty = styles.ApplyDesign(sty, "blueprint")
	sty.Design = "blueprint"
	rendered = ansi.Strip(item.Render(30))
	require.Contains(t, rendered, "YOU")
	require.Contains(t, rendered, "┌")
}

func TestDesignFrameHeaderDoesNotToggleThinking(t *testing.T) {
	sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
	msg := finishedAssistantMessage("reply", "A useful reply.")
	msg.Parts = append(msg.Parts, message.ReasoningContent{Thinking: "Checking source files"})
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.Render(80)
	require.Positive(t, item.thinkingBoxHeight)
	require.False(t, item.HandleMouseClick(ansi.MouseLeft, 2, 0))
	require.True(t, item.HandleMouseClick(ansi.MouseLeft, 2, 1))
}

func TestFullDesignsRespectNarrowWidth(t *testing.T) {
	for _, design := range []string{"cards", "dashboard", "studio", "neon", "terminal", "minimal", "focus"} {
		for _, width := range []int{0, 1, 2, 7, 8, 12, 80} {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
			sty.Design = design
			item := newTestUserItem(t, "Hello world with a longer line")
			item.sty = &sty
			assistant := NewAssistantMessageItem(&sty, finishedAssistantMessage("reply", "A useful reply.")).(*AssistantMessageItem)
			call := message.ToolCall{ID: "grep-1", Name: "grep", Finished: true, Input: `{"pattern":"hello"}`}
			tool := NewGrepToolMessageItem(&sty, call, &message.ToolResult{ToolCallID: call.ID, Content: "main.go:12: hello"}, false)
			for _, current := range []MessageItem{item, assistant, tool} {
				for _, line := range strings.Split(current.Render(width), "\n") {
					require.LessOrEqual(t, ansi.StringWidth(line), width, "%s width %d", design, width)
				}
				if width >= 8 && messageFramed(&sty, width) {
					require.Equal(t, len(strings.Split(current.RawRender(width), "\n")), len(strings.Split(current.Render(width), "\n")))
					if current == assistant {
						require.Contains(t, ansi.Strip(current.Render(width)), "RESP")
					}
				}
			}
		}
	}
}
