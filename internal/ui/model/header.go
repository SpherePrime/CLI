package model

import (
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/logo"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

const (
	headerDiag           = "╱"
	minHeaderDiags       = 3
	leftPadding          = 1
	rightPadding         = 1
	diagToDetailsSpacing = 1 // space between diagonal pattern and details section
)

type header struct {
	// frames caches the wide logo per shimmer step; compactFrames caches the
	// one-line wordmark used when the chat has focus in compact mode. Both are
	// keyed on gen, which refresh bumps whenever the theme is rebuilt.
	frames        logo.Frames
	compactFrames logo.Frames
	gen           uint64

	com *common.Common
}

// newHeader creates a new header model.
func newHeader(com *common.Common) *header {
	h := &header{
		com: com,
	}
	h.refresh()
	return h
}

// refresh marks every cached logo frame stale so the next draw rebuilds them
// with the current styles. Call after the theme changes.
func (h *header) refresh() {
	h.gen++
}

// wordmark returns the one-line "Prime™ PRIME" row for the compact header.
func (h *header) wordmark(t *styles.Styles, phase int) string {
	isHyper := h.com.IsHyper()
	label := "Prime™"
	if !isHyper {
		label = " " + label
	}
	name := "PRIME"
	if isHyper {
		name = "HYPERPRIME"
	}
	painted := styles.ApplyBoldForegroundGrad(t.Header.LogoGradCanvas, name, t.Header.LogoGradFromColor, t.Header.LogoGradToColor)
	if len(t.IridescentRamp) > 0 {
		painted = t.ApplyShimmer(t.Header.LogoGradCanvas, name, phase, 0, false, true)
	}
	return t.Header.Label.Render(label) + " " + painted + " "
}

// drawHeader draws the header for the given session. lspErrorCount comes
// from the UI's memoized LSP state: drawing runs on every frame and must not
// probe the workspace (a synchronous HTTP round-trip in client/server mode).
// phase is the shimmer step the wordmark walks through.
func (h *header) drawHeader(
	scr uv.Screen,
	area uv.Rectangle,
	session *session.Session,
	compact bool,
	detailsOpen bool,
	width int,
	lspErrorCount int,
	hyperCredits *int,
	phase int,
) {
	t := h.com.Styles
	hyper := h.com.IsHyper()
	if !compact || session == nil {
		key := fmt.Sprintf("wide|%d|%d|%t|%t", h.gen, width, compact, hyper)
		block := h.frames.Frame(key, phase, func(p int) string {
			return renderLogo(t, compact, hyper, width, p)
		})
		uv.NewStyledString(block).Draw(scr, area)
		return
	}

	if session.ID == "" {
		return
	}

	compactKey := fmt.Sprintf("compact|%d|%t", h.gen, hyper)
	wordmark := h.compactFrames.Frame(compactKey, phase, func(p int) string {
		return h.wordmark(t, p)
	})

	var b strings.Builder
	b.WriteString(wordmark)

	availDetailWidth := width - leftPadding - rightPadding - lipgloss.Width(b.String()) - minHeaderDiags - diagToDetailsSpacing
	details := renderHeaderDetails(
		h.com,
		session,
		lspErrorCount,
		detailsOpen,
		availDetailWidth,
		hyperCredits,
	)

	remainingWidth := width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding -
		diagToDetailsSpacing

	if remainingWidth > 0 {
		diagonals := strings.Repeat(headerDiag, max(minHeaderDiags, remainingWidth))
		startCell := lipgloss.Width(b.String())
		b.WriteString(h.shimmerDiagonals(t, diagonals, phase, startCell))
		b.WriteString(" ")
	}

	b.WriteString(details)

	view := uv.NewStyledString(
		t.Header.Wrapper.Padding(0, rightPadding, 0, leftPadding).Render(b.String()),
	)
	view.Draw(scr, area)
}

// shimmerDiagonals paints the compact header's diagonal rule with the theme's
// muted spectrum, continuing where the wordmark left off so the two read as
// one strip.
func (h *header) shimmerDiagonals(t *styles.Styles, diagonals string, phase, startCell int) string {
	return t.ApplyShimmer(t.Header.Diagonals, diagonals, phase, startCell, true, false)
}

// renderHeaderDetails renders the details section of the header.
func renderHeaderDetails(
	com *common.Common,
	session *session.Session,
	lspErrorCount int,
	detailsOpen bool,
	availWidth int,
	hyperCredits *int,
) string {
	t := com.Styles

	var parts []string

	if lspErrorCount > 0 {
		parts = append(parts, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPErrorIcon, lspErrorCount)))
	}

	agentCfg := com.Config().Agents[config.AgentCoder]
	model := com.Config().GetModelByType(agentCfg.Model)
	if model != nil {
		sel, ok := com.Config().Models[agentCfg.Model]
		if ok && sel.ContextWindow > 0 {
			model.ContextWindow = sel.ContextWindow
		}
	}
	if model != nil && model.ContextWindow > 0 {
		percentage := (float64(session.CompletionTokens+session.PromptTokens) / float64(model.ContextWindow)) * 100
		percentageText := fmt.Sprintf("%d%%", int(percentage))
		if session.EstimatedUsage {
			percentageText = "~" + percentageText
		}
		maxContextText := common.FormatContextTokens(model.ContextWindow)
		contextDetail := fmt.Sprintf("%s (%s)", percentageText, maxContextText)
		parts = append(parts, t.Header.Percentage.Render(contextDetail))
	}

	if com.IsHyper() && hyperCredits != nil {
		hc := t.Header.HypercreditIcon.Render(styles.HypercreditIcon) + " " + t.Header.Percentage.Render(common.FormatCredits(*hyperCredits))
		parts = append(parts, hc)
	}

	const keystroke = "ctrl+d"
	if detailsOpen {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" close"))
	} else {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" open "))
	}

	dot := t.Header.Separator.Render(" • ")
	metadata := strings.Join(parts, dot)
	metadata = dot + metadata

	const dirTrimLimit = 4
	cwd := fsext.DirTrim(fsext.PrettyPath(com.Workspace.WorkingDir()), dirTrimLimit)
	cwd = t.Header.WorkingDir.Render(cwd)

	result := cwd + metadata
	return ansi.Truncate(result, max(0, availWidth), "…")
}
