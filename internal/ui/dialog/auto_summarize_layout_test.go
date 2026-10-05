package dialog

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// renderedRow builds the row exactly as Draw does, so this measures the real
// thing rather than re-deriving the arithmetic and getting it wrong in the same
// way twice.
//
// The previous version of this test compared the raw label and suffix lengths
// against the dialog width and passed, while the dialog was visibly broken:
// the styles carry two cells of horizontal padding each, so a row that measured
// 68 unstyled came out at 72 styled and lipgloss wrapped the suffix onto a line
// of its own. Checking the strings cannot see that.
func renderedRow(a *AutoSummarize, row *autoSummarizeField, inner int, focused bool) string {
	t := a.com.Styles

	labelStyle, valueStyle := t.Dialog.NormalItem, t.Dialog.ListItem.InfoBlurred
	if focused {
		labelStyle, valueStyle = t.Dialog.SelectedItem, t.Dialog.ListItem.InfoFocused
	}

	label := padRightTo(truncateToWidth(row.title, autoSummarizeLabelColumn), autoSummarizeLabelColumn)
	labelRendered := labelStyle.Render(label)
	suffixRendered := t.Dialog.NormalItem.Render(" " + row.suffix)

	available := inner - lipgloss.Width(labelRendered) - lipgloss.Width(suffixRendered)
	row.input.SetWidth(dialogInputTextWidth(t, row.input, max(available, 4)))

	return labelRendered + valueStyle.Render(row.input.View()) + suffixRendered
}

// A row is one line: label, value, suffix. If it is not, the suffix ends up on a
// line of its own and a single field reads as three lines with a bare "%" or
// "tokens" stranded between the value and its description.
func TestAutoSummarizeRowStaysOnOneLine(t *testing.T) {
	for _, locale := range []string{i18n.En, i18n.Ru} {
		t.Run(locale, func(t *testing.T) {
			a := newAutoSummarizeTestDialog(t, &config.Options{}, locale)
			inner := autoSummarizeDialogMaxWidth - a.com.Styles.Dialog.View.GetHorizontalFrameSize()

			for i, row := range a.rows {
				for _, focused := range []bool{false, true} {
					out := renderedRow(a, row, inner, focused)

					require.Equal(t, 1, strings.Count(out, "\n")+1,
						"row %d (%s, focused=%v) wrapped into more than one line:\n%q",
						i, row.key, focused, out)
					require.LessOrEqual(t, lipgloss.Width(out), inner,
						"row %d (%s, focused=%v) is wider than the dialog: %d > %d",
						i, row.key, focused, lipgloss.Width(out), inner)
				}
			}
		})
	}
}

// The suffix has to stay on the same line as the value, right after it.
// Checking only that the row did not wrap would still pass with the suffix
// somewhere harmless-looking, and the value would read as a bare number with no
// unit.
//
// Trailing space is trimmed because the style pads the suffix by a cell on each
// side; the assertion is about which line the suffix ended up on.
func TestAutoSummarizeSuffixStaysNextToTheValue(t *testing.T) {
	for _, locale := range []string{i18n.En, i18n.Ru} {
		t.Run(locale, func(t *testing.T) {
			a := newAutoSummarizeTestDialog(t, &config.Options{}, locale)
			inner := autoSummarizeDialogMaxWidth - a.com.Styles.Dialog.View.GetHorizontalFrameSize()

			for _, row := range a.rows {
				bare := strings.TrimRight(ansi.Strip(renderedRow(a, row, inner, true)), " ")

				require.True(t, strings.HasSuffix(bare, row.suffix),
					"row %q must end with its suffix %q on the same line, got %q",
					row.key, row.suffix, bare)
			}
		})
	}
}

func TestAutoSummarizeDescriptionFitsOnOneLine(t *testing.T) {
	for _, locale := range []string{i18n.En, i18n.Ru} {
		t.Run(locale, func(t *testing.T) {
			a := newAutoSummarizeTestDialog(t, &config.Options{}, locale)
			inner := autoSummarizeDialogMaxWidth - a.com.Styles.Dialog.View.GetHorizontalFrameSize()

			for _, row := range a.rows {
				out := a.com.Styles.Dialog.NormalItem.Render(truncateToWidth(row.description, inner))
				require.Equal(t, 1, strings.Count(out, "\n")+1,
					"description for %q wraps: %q", row.key, out)
			}
		})
	}
}