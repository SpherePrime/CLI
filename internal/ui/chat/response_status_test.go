package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestResponseStatusRemainsReadableAcrossDesigns(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		t.Run(design, func(t *testing.T) {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), design)
			sty.Design = design
			item := NewAssistantMessageItem(&sty, &message.Message{ID: "running", Role: message.Assistant}).(*AssistantMessageItem)
			for range 30 {
				status := ansi.Strip(item.renderSpinning())
				require.Contains(t, status, "Working")
				for _, cell := range []rune(status)[:10] {
					require.Contains(t, "●·▁▂▃▄▅▆▇█▰▱░▒▓╱╲│─[]>-┃✎•▧▣˙✧✦ ", string(cell))
				}
				item.Advance()
			}
		})
	}
}

func TestResponseFooterClearsDesignCache(t *testing.T) {
	sty := styles.ApplyDesign(styles.ColorTonePantera(), "classic")
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	item := NewAssistantInfoItem(&sty, finishedAssistantMessage("finished", "Done"), cfg, time.Unix(0, 0)).(*AssistantInfoItem)
	item.Render(80)
	sty = styles.ApplyDesign(sty, "terminal")
	rendered := ansi.Strip(item.Render(80))
	require.True(t, strings.HasPrefix(rendered, "$ [DONE] "))
	require.NotContains(t, rendered, "─")
}

func TestResponseStatusUsesCurrentState(t *testing.T) {
	sty := styles.ApplyDesign(styles.ColorTonePantera(), "cards")
	msg := thinkingMessage("thinking", "checking", "")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	require.Contains(t, ansi.Strip(item.renderSpinning()), "Thinking")
	item.message = &message.Message{ID: "summary", Role: message.Assistant, IsSummaryMessage: true}
	require.Contains(t, ansi.Strip(item.renderSpinning()), "Summarizing")
	item.message = &message.Message{ID: "working", Role: message.Assistant}
	require.Contains(t, ansi.Strip(item.renderSpinning()), "Working")
}

func TestAdditionalDesignResponseStatus(t *testing.T) {
	for _, test := range []struct{ design, working, footer string }{
		{"opencode", "┃┃││││││││", "│ DONE · "},
		{"paper", "✎ •·······", "· "},
		{"blueprint", "▣▧▧▧▧▧▧▧▧▧", "┌ DONE "},
		{"ember", "˙·✧✦✧·˙·✧·", "▌ DONE · "},
	} {
		t.Run(test.design, func(t *testing.T) {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), test.design)
			sty.Design = test.design
			item := NewAssistantMessageItem(&sty, &message.Message{ID: "running", Role: message.Assistant}).(*AssistantMessageItem)
			require.True(t, strings.HasPrefix(ansi.Strip(item.renderSpinning()), test.working))
			cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
			info := NewAssistantInfoItem(&sty, finishedAssistantMessage("finished", "Done"), cfg, time.Unix(0, 0)).(*AssistantInfoItem)
			require.True(t, strings.HasPrefix(ansi.Strip(info.Render(80)), test.footer))
		})
	}
}

func TestResponseFooterFollowsDesignAndWidth(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		t.Run(design, func(t *testing.T) {
			sty := styles.ApplyDesign(styles.ColorTonePantera(), design)
			sty.Design = design
			msg := finishedAssistantMessage("finished", "Done")
			cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
			item := NewAssistantInfoItem(&sty, msg, cfg, time.Unix(0, 0)).(*AssistantInfoItem)
			for _, width := range []int{0, 1, 2, 7, 8, 20, 80} {
				rendered := ansi.Strip(item.Render(width))
				for _, line := range strings.Split(rendered, "\n") {
					require.LessOrEqual(t, ansi.StringWidth(line), width)
				}
				if width == 80 && messageFramed(&sty, width) {
					require.Contains(t, rendered, "DONE")
					require.NotContains(t, ansi.Strip(item.RawRender(width)), "DONE")
					require.Equal(t, len(strings.Split(rendered, "\n")), len(strings.Split(item.RawRender(width), "\n")))
				}
			}
		})
	}
}
