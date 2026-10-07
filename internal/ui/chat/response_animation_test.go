package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestResponseAnimationsMoveWithStableDimensions(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		t.Run(design, func(t *testing.T) {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), design)
			item := NewAssistantMessageItem(&sty, &message.Message{ID: "running", Role: message.Assistant}).(*AssistantMessageItem)
			first := ansi.Strip(item.Render(80))
			frames := map[string]bool{first: true}
			for range 30 {
				require.True(t, item.Advance())
				rendered := ansi.Strip(item.Render(80))
				frames[rendered] = true
				require.Contains(t, rendered, "Working")
				require.Equal(t, lipgloss.Width(first), lipgloss.Width(rendered))
				require.Equal(t, lipgloss.Height(first), lipgloss.Height(rendered))
			}
			require.GreaterOrEqual(t, len(frames), 4, "animation must move without waiting for the elapsed timer")
		})
	}
}

func TestResponseAnimationCyclesRemainDistinctAndDeterministic(t *testing.T) {
	seen := make(map[string]bool)
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		sty := styles.ResolveTheme("", "", design)
		item := NewAssistantMessageItem(&sty, &message.Message{ID: "running", Role: message.Assistant}).(*AssistantMessageItem)
		first := ansi.Strip(item.renderSpinning())
		require.False(t, seen[first], "%s must have its own animation", design)
		seen[first] = true
		for range 360 {
			before := item.renderSpinning()
			require.Equal(t, before, item.renderSpinning(), "rendering without an animation tick must remain stable")
			item.Advance()
		}
		require.Equal(t, first, ansi.Strip(item.renderSpinning()), "%s must loop", design)
		item.clearCache()
		require.Equal(t, first, ansi.Strip(item.renderSpinning()))
		sty = styles.ResolveTheme("", "", "cards")
		item.Render(80)
		require.Contains(t, ansi.Strip(item.renderSpinning()), "▰▰")
		item.SetMessage(finishedAssistantMessage("running", "Done"))
		require.False(t, item.Advance())
		require.NotContains(t, ansi.Strip(item.Render(80)), "Working")
	}
}

func TestResponseAnimationTimerDoesNotResize(t *testing.T) {
	sty := styles.ColorTonePantera()
	for _, state := range []string{"Working", "Thinking", "Summarizing"} {
		for _, elapsed := range []string{"", "9s", "10s", "1m 20s", "10h 20m"} {
			rendered := renderResponseAnimation(&sty, state, 0, 80, elapsed)
			require.Equal(t, 31, ansi.StringWidth(rendered))
			require.Contains(t, ansi.Strip(rendered), state)
		}
	}
}

func TestExportResponseAnimationFrames(t *testing.T) {
	directory := os.Getenv("PRIME_ANIMATION_PREVIEW_DIR")
	if directory == "" {
		t.Skip()
	}
	require.NoError(t, os.MkdirAll(directory, 0o755))
	designs := []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"}
	for frame := range 120 {
		var rows []string
		for pair := range 6 {
			left := styles.ResolveTheme("", "", designs[pair*2])
			right := styles.ResolveTheme("", "", designs[pair*2+1])
			format := func(sty *styles.Styles, text string) string {
				return styles.WithBackground(lipgloss.NewStyle().Foreground(sty.Tool.NameNormal.GetForeground()).Width(80).Render(text), sty.Background)
			}
			labelLeft := format(&left, "  "+strings.ToUpper(left.Design))
			labelRight := format(&right, "  "+strings.ToUpper(right.Design))
			elapsed := fmt.Sprintf("%ds", frame*3/20)
			statusLeft := format(&left, "  "+renderResponseAnimation(&left, "Working", uint64(frame*3), 75, elapsed))
			statusRight := format(&right, "  "+renderResponseAnimation(&right, "Working", uint64(frame*3), 75, elapsed))
			blank := format(&left, "") + format(&right, "")
			rows = append(rows, blank, labelLeft+labelRight, blank, statusLeft+statusRight, blank, blank)
		}
		require.NoError(t, os.WriteFile(filepath.Join(directory, fmt.Sprintf("frame-%03d.ansi", frame)), []byte(strings.Join(rows, "\n")), 0o644))
	}
}

func TestResponseAnimationsRespectNarrowWidth(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		sty := styles.ApplyDesign(styles.ColorTonePantera(), design)
		item := NewAssistantMessageItem(&sty, &message.Message{ID: "running", Role: message.Assistant}).(*AssistantMessageItem)
		for width := range 21 {
			for range 6 {
				item.Advance()
				for _, rendered := range []string{item.RawRender(width), item.Render(width)} {
					for _, line := range strings.Split(rendered, "\n") {
						require.LessOrEqual(t, ansi.StringWidth(line), width, "%s width %d", design, width)
					}
				}
			}
		}
	}
}
