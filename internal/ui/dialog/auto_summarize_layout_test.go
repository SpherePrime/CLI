package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Every row has to fit the dialog on one line: label, value, suffix. When the
// suffix was not subtracted from the input width, lipgloss wrapped it onto a
// line of its own and a single field read as three lines, with a bare "%"
// stranded between the value and the description.
func TestAutoSummarizeRowFitsDialogWidth(t *testing.T) {
	for _, locale := range []string{i18n.En, i18n.Ru} {
		t.Run(locale, func(t *testing.T) {
			a := newAutoSummarizeTestDialog(t, &config.Options{}, locale)

			sty := a.com.Styles
			width := autoSummarizeDialogMaxWidth
			inner := width - sty.Dialog.View.GetHorizontalFrameSize()

			for _, row := range a.rows {
				label := padRightTo(truncateToWidth(row.title, autoSummarizeLabelColumn), autoSummarizeLabelColumn)
				suffix := " " + row.suffix

				require.LessOrEqual(t,
					lipgloss.Width(label)+lipgloss.Width(suffix)+2, inner,
					"row %q: label and suffix must leave room for the value", row.key)

				// And the description has to fit too, or it becomes a second
				// paragraph under a field that looks like it wrapped.
				require.LessOrEqual(t,
					lipgloss.Width(truncateToWidth(row.description, inner)), inner,
					"row %q: description overflows the dialog", row.key)
			}
		})
	}
}
