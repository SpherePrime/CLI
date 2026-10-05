package dialog

import (
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

const (
	// AutoSummarizeID is the identifier for the auto-summarize thresholds dialog.
	AutoSummarizeID             = "auto_summarize"
	autoSummarizeDialogMaxWidth = 72
	// Wide enough for the longest label in either language. "Сжимать при
	// заполнении" is 22 characters, and a label that wraps makes the row
	// unreadable because the value ends up on the next line.
	autoSummarizeLabelColumn     = 23
	autoSummarizeWindowStep      = 1000
	autoSummarizeMaxWindowTokens = 100_000_000
)

// autoSummarizeField is one editable threshold.
//
// The dialog is a small form rather than a picker because every value is a
// number. Offering a fixed set of choices could not express "compact at 87%
// of this window", and the user asked to be able to set the values.
type autoSummarizeField struct {
	// key is the options path written back to the config file.
	key string
	// title, description and suffix are the labels shown in the row.
	title       string
	description string
	suffix      string
	// min and max bound the accepted value.
	min, max int
	// step is how much the increment keys move the value.
	step int
	// value is the last known good value, used when the field cannot be
	// parsed or falls outside the bounds.
	value int
	// input is the text field bound to this row.
	input textinput.Model
}

// parsedOr returns the field's current value when it is a valid number, and
// fallback otherwise, so a half-typed or out-of-range entry never leaks into
// the resolved policy.
func (r *autoSummarizeField) parsedOr(fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(r.input.Value()))
	if err != nil {
		return fallback
	}
	return min(max(n, r.min), r.max)
}

// AutoSummarize edits the thresholds that decide when a conversation is
// compacted automatically. It is reached from the command palette next to the
// other settings menus.
type AutoSummarize struct {
	com   *common.Common
	help  help.Model
	rows  []*autoSummarizeField
	focus int

	keyMap struct {
		Up     key.Binding
		Down   key.Binding
		Dec    key.Binding
		Inc    key.Binding
		Save   key.Binding
		Reset  key.Binding
		Close  key.Binding
	}
}

var (
	_ Dialog         = (*AutoSummarize)(nil)
	_ BackspaceAware = (*AutoSummarize)(nil)
)

// NewAutoSummarize creates the auto-summarize thresholds dialog.
func NewAutoSummarize(com *common.Common) *AutoSummarize {
	a := &AutoSummarize{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	a.help = h

	a.keyMap.Up = key.NewBinding(
		key.WithKeys("up", "shift+tab"),
		key.WithHelp("↑", "previous field"),
	)
	a.keyMap.Down = key.NewBinding(
		key.WithKeys("down", "tab"),
		key.WithHelp("↓", "next field"),
	)
	a.keyMap.Dec = key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "decrease"),
	)
	a.keyMap.Inc = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "increase"),
	)
	a.keyMap.Save = key.NewBinding(
		key.WithKeys("enter", "ctrl+s"),
		key.WithHelp("enter", "save"),
	)
	a.keyMap.Reset = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "defaults"),
	)
	a.keyMap.Close = CloseKey

	a.setRows()
	return a
}

// ID implements Dialog.
func (a *AutoSummarize) ID() string {
	return AutoSummarizeID
}

// RefreshLocale retranslates the row labels after the UI language changes.
func (a *AutoSummarize) RefreshLocale() {
	a.setRows()
}

// policy resolves the thresholds currently in effect, which is what the rows
// are seeded from and what the summary line reports.
func (a *AutoSummarize) policy() config.AutoSummarizePolicy {
	cfg := a.com.Config()
	if cfg == nil {
		return config.DefaultAutoSummarizePolicy()
	}
	return cfg.Options.GetAutoSummarizePolicy()
}

// activeContextWindow reports the window the coder agent's model declares,
// which is the window these thresholds will actually be applied to.
func (a *AutoSummarize) activeContextWindow() int64 {
	cfg := a.com.Config()
	if cfg == nil {
		return 0
	}
	agentCfg, ok := cfg.Agents[config.AgentGeneral]
	if !ok {
		return 0
	}
	model := cfg.GetModelByType(agentCfg.Model)
	if model == nil {
		return 0
	}
	return int64(model.ContextWindow)
}

// setRows rebuilds the form from the current config, keeping any half-typed
// value the user already entered for the same field.
func (a *AutoSummarize) setRows() {
	t := a.com.Styles
	policy := a.policy()
	prevFocus := a.focus
	prevRows := a.rows

	rows := []*autoSummarizeField{
		{
			key:         "options.auto_summarize_percent",
			title:       a.com.L("cmd.auto_summarize_percent"),
			description: a.com.L("cmd.auto_summarize_percent_desc"),
			suffix:      "%",
			min:         50,
			max:         99,
			step:        1,
			value:       policy.Percent,
		},
		{
			key:         "options.auto_summarize_large_percent",
			title:       a.com.L("cmd.auto_summarize_large_percent"),
			description: a.com.L("cmd.auto_summarize_large_percent_desc"),
			suffix:      "%",
			min:         50,
			max:         99,
			step:        1,
			value:       policy.LargePercent,
		},
		{
			key:         "options.auto_summarize_large_window",
			title:       a.com.L("cmd.auto_summarize_large_window"),
			description: a.com.L("cmd.auto_summarize_large_window_desc"),
			suffix:      a.com.L("cmd.auto_summarize_tokens"),
			min:         autoSummarizeWindowStep,
			max:         autoSummarizeMaxWindowTokens,
			step:        autoSummarizeWindowStep,
			value:       int(policy.LargeWindow),
		},
	}

	for i, row := range rows {
		if i < len(prevRows) && prevRows[i].key == row.key && prevRows[i].input.Value() != "" {
			// Preserve the live field, and treat its text as the new baseline
			// so an invalid entry falls back to what the user typed rather
			// than to the old saved value.
			row.value = prevRows[i].parsedOr(row.value)
			row.input = prevRows[i].input
			continue
		}
		row.input = textinput.New()
		row.input.SetVirtualCursor(false)
		row.input.CharLimit = 9
		row.input.SetStyles(t.TextInput)
		row.input.SetValue(strconv.Itoa(row.value))
	}

	a.rows = rows
	if prevFocus >= len(rows) {
		prevFocus = len(rows) - 1
	}
	a.focus = max(0, prevFocus)
	a.focusInput()
}

// focusInput gives the caret to the selected row only, so typing always lands
// where the highlight is.
func (a *AutoSummarize) focusInput() {
	for i, row := range a.rows {
		if i == a.focus {
			row.input.Focus()
			row.input.CursorEnd()
			continue
		}
		row.input.Blur()
	}
}

// HandleMsg implements [Dialog].
func (a *AutoSummarize) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, a.keyMap.Up):
			a.moveFocus(-1)
			return nil
		case key.Matches(msg, a.keyMap.Down):
			a.moveFocus(1)
			return nil
		case key.Matches(msg, a.keyMap.Save):
			return a.save()
		case key.Matches(msg, a.keyMap.Reset):
			a.reset()
			return nil
		case key.Matches(msg, a.keyMap.Dec):
			a.adjust(-1)
			return nil
		case key.Matches(msg, a.keyMap.Inc):
			a.adjust(1)
			return nil
		}

		row := a.rows[a.focus]
		var cmd tea.Cmd
		row.input, cmd = row.input.Update(msg)
		return ActionCmd{cmd}
	case tea.PasteMsg:
		row := a.rows[a.focus]
		var cmd tea.Cmd
		row.input, cmd = row.input.Update(msg)
		return ActionCmd{cmd}
	}
	return nil
}

func (a *AutoSummarize) moveFocus(delta int) {
	next := min(max(a.focus+delta, 0), len(a.rows)-1)
	if next == a.focus {
		return
	}
	a.focus = next
	a.focusInput()
}

// adjust nudges the focused field by its step, which is faster than retyping
// a number when only the last digit is wrong.
//
// The result is clamped, not just the input it reads: clamping only the base
// value would let a field already sitting at its maximum step straight past
// it, which save would then reject.
func (a *AutoSummarize) adjust(delta int) {
	row := a.rows[a.focus]
	next := row.parsedOr(row.value) + delta*row.step
	row.input.SetValue(strconv.Itoa(min(max(next, row.min), row.max)))
}

// reset puts every field back to the shipped defaults.
func (a *AutoSummarize) reset() {
	defaults := config.DefaultAutoSummarizePolicy()
	values := []int{defaults.Percent, defaults.LargePercent, int(defaults.LargeWindow)}
	for i, row := range a.rows {
		row.value = values[i]
		row.input.SetValue(strconv.Itoa(values[i]))
	}
}

// save validates every row before emitting anything, so a rejected value
// cannot leave the thresholds half applied.
func (a *AutoSummarize) save() Action {
	values := make([]int, len(a.rows))
	for i, row := range a.rows {
		raw := strings.TrimSpace(row.input.Value())
		n, err := strconv.Atoi(raw)
		if err != nil {
			return ActionShowError{Message: a.com.LSprintf("cmd.auto_summarize_not_a_number", row.title, raw)}
		}
		if n < row.min || n > row.max {
			return ActionShowError{Message: a.com.LSprintf("cmd.auto_summarize_out_of_range", row.title, row.min, row.max)}
		}
		values[i] = n
	}

	return ActionSetAutoSummarizeThresholds{
		Percent:      values[0],
		LargePercent: values[1],
		LargeWindow:  int64(values[2]),
	}
}

// BackspaceDeletesText implements [BackspaceAware].
func (a *AutoSummarize) BackspaceDeletesText() bool {
	return BackspaceDeletesText(a.rows[a.focus].input)
}

// Cursor returns the cursor position relative to the dialog.
func (a *AutoSummarize) Cursor() *tea.Cursor {
	return InputCursor(a.com.Styles, a.rows[a.focus].input.Cursor())
}

// Draw implements [Dialog].
func (a *AutoSummarize) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := a.com.Styles
	width := max(0, min(autoSummarizeDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())

	rc := NewRenderContext(t, width)
	rc.Title = a.com.L("cmd.auto_summarize_title")

	for i, row := range a.rows {
		focused := i == a.focus

		labelStyle := t.Dialog.NormalItem
		valueStyle := t.Dialog.ListItem.InfoBlurred
		descStyle := t.Dialog.NormalItem
		if focused {
			labelStyle = t.Dialog.SelectedItem
			valueStyle = t.Dialog.ListItem.InfoFocused
			descStyle = t.Dialog.ListItem.InfoFocused
		}

		// Budget the value against the width the label and suffix actually
		// take once styled, not against their raw lengths.
		//
		// The styles carry two cells of horizontal padding each, so measuring
		// the strings gave 68 cells for a 70 cell row: it looked like it fit,
		// and the styled result came out at 72. That overflow is what wrapped
		// the suffix onto a line of its own, leaving a bare "%" stranded
		// between the value and its description. Measuring the rendered parts
		// cannot drift when a theme changes the padding.
		label := padRightTo(truncateToWidth(row.title, autoSummarizeLabelColumn), autoSummarizeLabelColumn)
		labelRendered := labelStyle.Render(label)
		suffixRendered := t.Dialog.NormalItem.Render(" " + row.suffix)

		available := innerWidth - lipgloss.Width(labelRendered) - lipgloss.Width(suffixRendered)
		row.input.SetWidth(dialogInputTextWidth(t, row.input, max(available, 4)))

		rc.AddPart(labelRendered + valueStyle.Render(row.input.View()) + suffixRendered)
		rc.AddPart(descStyle.Render(truncateToWidth(row.description, innerWidth)))
	}

	if summary := a.summaryLine(); summary != "" {
		rc.AddPart(t.Dialog.NormalItem.Render(summary))
	}

	rc.Help = renderDialogHelp(t, &a.help, a, innerWidth)

	view := rc.Render()
	cur := a.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// editedPolicy is the policy the fields currently describe, falling back to the
// last valid value per field for anything half-typed or out of range.
//
// It has to be built from the fields rather than read back from the config: the
// summary line says which threshold will actually apply, and reading the config
// there made it describe the saved state while the user was editing a new one.
// So changing 85 to 90 left the line claiming 85, which is the one number in the
// dialog a user checks to see whether their edit took effect.
func (a *AutoSummarize) editedPolicy() config.AutoSummarizePolicy {
	policy := a.policy()
	for _, row := range a.rows {
		v := row.parsedOr(row.value)
		switch row.key {
		case "options.auto_summarize_percent":
			policy.Percent = v
		case "options.auto_summarize_large_percent":
			policy.LargePercent = v
		case "options.auto_summarize_large_window":
			policy.LargeWindow = int64(v)
		}
	}
	return policy
}

// summaryLine reports the threshold that will actually apply to the model in
// use, so the two percentages are not just numbers in isolation.
func (a *AutoSummarize) summaryLine() string {
	cw := a.activeContextWindow()
	if cw <= 0 {
		return a.com.L("cmd.auto_summarize_unknown_window")
	}
	return a.com.LSprintf("cmd.auto_summarize_summary", cw, a.editedPolicy().SummarizeAt(cw))
}

// ShortHelp implements [help.KeyMap].
func (a *AutoSummarize) ShortHelp() []key.Binding {
	return []key.Binding{a.keyMap.Up, a.keyMap.Down, a.keyMap.Save, a.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (a *AutoSummarize) FullHelp() [][]key.Binding {
	slice := []key.Binding{a.keyMap.Up, a.keyMap.Down, a.keyMap.Dec, a.keyMap.Inc, a.keyMap.Save, a.keyMap.Reset, a.keyMap.Close}
	m := [][]key.Binding{}
	for i := 0; i < len(slice); i += 4 {
		m = append(m, slice[i:min(i+4, len(slice))])
	}
	return m
}

// truncateToWidth shortens s to width cells, marking the cut with an ellipsis.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

// padRightTo pads s with spaces so the next column starts at width.
func padRightTo(s string, width int) string {
	if pad := width - len([]rune(s)); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
