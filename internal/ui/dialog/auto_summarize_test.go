package dialog

import (
	"strconv"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// rowValues reads the current text out of each field.
func rowValues(a *AutoSummarize) []string {
	out := make([]string, len(a.rows))
	for i, row := range a.rows {
		out[i] = row.input.Value()
	}
	return out
}

// newAutoSummarizeTestDialog builds the form over a stub workspace so the
// thresholds can be read back without touching the real config file.
func newAutoSummarizeTestDialog(t *testing.T, opts *config.Options, locale ...string) *AutoSummarize {
	t.Helper()
	return newAutoSummarizeTestDialogWithWindow(t, opts, 0, locale...)
}

// newAutoSummarizeTestDialogWithWindow also configures the main agent's model
// with a context window, so the summary line has something to report. Without
// one it says the model declares no window, which is a different code path.
func newAutoSummarizeTestDialogWithWindow(t *testing.T, opts *config.Options, window int64, locale ...string) *AutoSummarize {
	t.Helper()

	lang := i18n.En
	if len(locale) > 0 {
		lang = locale[0]
	}

	cfg := config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Options:   opts,
	}

	if window > 0 {
		cfg.Agents = map[string]config.Agent{}
		cfg.SetupAgents()
		general := cfg.Agents[config.AgentGeneral]
		general.Model = config.SelectedModelTypeLarge
		cfg.Agents[config.AgentGeneral] = general

		cfg.Models = map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "p", Model: "m"},
		}
		cfg.Providers.Set("p", config.ProviderConfig{
			ID:     "p",
			Models: []catwalk.Model{{ID: "m", ContextWindow: window}},
		})
	}

	ws := &autoSummarizeTestWorkspace{cfg: cfg, locale: lang}
	return NewAutoSummarize(&common.Common{
		Workspace: ws,
		Styles:    providerTestStyles(),
	})
}

type autoSummarizeTestWorkspace struct {
	workspace.Workspace
	cfg         config.Config
	locale      string
	writtenKeys map[string]any
}

func (w *autoSummarizeTestWorkspace) Config() *config.Config { return &w.cfg }

// Language pins the catalog so the assertions do not depend on the developer's
// configured UI language.
func (w *autoSummarizeTestWorkspace) Language() string { return w.locale }

func (w *autoSummarizeTestWorkspace) SetConfigFields(scope config.Scope, kv map[string]any) error {
	if w.writtenKeys == nil {
		w.writtenKeys = map[string]any{}
	}
	for k, v := range kv {
		w.writtenKeys[k] = v
	}
	return nil
}

func TestAutoSummarizeSeedsFromConfig(t *testing.T) {
	t.Parallel()

	percent := 70
	largePercent := 99
	largeWindow := int64(1_000_000)

	a := newAutoSummarizeTestDialog(t, &config.Options{
		AutoSummarizePercent:      &percent,
		AutoSummarizeLargePercent: &largePercent,
		AutoSummarizeLargeWindow:  &largeWindow,
	})

	require.Equal(t, []string{"70", "99", "1000000"}, rowValues(a),
		"the form must show the configured thresholds, not the defaults")
}

func TestAutoSummarizeDefaultsWhenUnset(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	require.Equal(t, []string{
		strconv.Itoa(config.DefaultAutoSummarizePercent),
		strconv.Itoa(config.DefaultAutoSummarizeLargePercent),
		strconv.Itoa(config.DefaultAutoSummarizeLargeWindow),
	}, rowValues(a))
}

func TestAutoSummarizeSaveEmitsThresholds(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.rows[0].input.SetValue("88")
	a.rows[1].input.SetValue("97")
	a.rows[2].input.SetValue("640000")

	action := a.save()
	thresholds, ok := action.(ActionSetAutoSummarizeThresholds)
	require.True(t, ok, "save must emit the thresholds action, got %T", action)
	require.Equal(t, 88, thresholds.Percent)
	require.Equal(t, 97, thresholds.LargePercent)
	require.Equal(t, int64(640_000), thresholds.LargeWindow)
}

// A rejected value must not emit a partial write. The percentages only make
// sense as a set, so half-applying them would be worse than not applying them.
func TestAutoSummarizeSaveRejectsBadValues(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		row   int
		value string
	}{
		{"not a number", 0, "abc"},
		{"empty", 0, ""},
		{"float", 0, "85.5"},
		{"below the floor", 0, "10"},
		{"above the ceiling", 0, "100"},
		{"negative", 1, "-5"},
		{"window too small", 2, "10"},
		{"window too large", 2, "999999999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newAutoSummarizeTestDialog(t, &config.Options{})

			a.rows[tc.row].input.SetValue(tc.value)

			action := a.save()
			require.IsType(t, ActionShowError{}, action,
				"an invalid value must surface an error instead of saving")
		})
	}
}

func TestAutoSummarizeSaveAcceptsBounds(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.rows[0].input.SetValue("50")
	a.rows[1].input.SetValue("99")

	action := a.save()
	require.IsType(t, ActionSetAutoSummarizeThresholds{}, action,
		"the documented bounds themselves must be accepted")
}

func TestAutoSummarizeResetRestoresDefaults(t *testing.T) {
	t.Parallel()

	percent := 70
	opts := &config.Options{AutoSummarizePercent: &percent}
	a := newAutoSummarizeTestDialog(t, opts)

	a.rows[0].input.SetValue("60")
	a.rows[2].input.SetValue("123")
	a.reset()

	require.Equal(t, []string{
		strconv.Itoa(config.DefaultAutoSummarizePercent),
		strconv.Itoa(config.DefaultAutoSummarizeLargePercent),
		strconv.Itoa(config.DefaultAutoSummarizeLargeWindow),
	}, rowValues(a))
}

// Ctrl+u and ctrl+d nudge by the row's step, which is 1000 tokens for the
// window field and a single percent for the two percentages.
func TestAutoSummarizeAdjustUsesPerRowStep(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.focus = 0
	a.adjust(1)
	require.Equal(t, strconv.Itoa(config.DefaultAutoSummarizePercent+1), a.rows[0].input.Value())

	a.adjust(-1)
	require.Equal(t, strconv.Itoa(config.DefaultAutoSummarizePercent), a.rows[0].input.Value())

	a.focus = 2
	a.adjust(1)
	require.Equal(t, strconv.Itoa(config.DefaultAutoSummarizeLargeWindow+1000), a.rows[2].input.Value())
}

func TestAutoSummarizeAdjustClampsToBounds(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.focus = 0
	a.rows[0].input.SetValue("99")
	a.adjust(1)
	require.Equal(t, "99", a.rows[0].input.Value())

	a.rows[0].input.SetValue("50")
	a.adjust(-1)
	require.Equal(t, "50", a.rows[0].input.Value())
}

func TestAutoSummarizeFocusMovesWithinBounds(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.moveFocus(-1)
	require.Equal(t, 0, a.focus)

	a.moveFocus(1)
	require.Equal(t, 1, a.focus)

	a.moveFocus(99)
	require.Equal(t, len(a.rows)-1, a.focus)
	require.NotEmpty(t, a.rows[a.focus].input.Focused(), "the caret must follow the selection")
}

// A half-typed value has to survive a locale change, which rebuilds the rows.
func TestAutoSummarizeKeepsTypedValueAcrossRebuild(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.rows[0].input.SetValue("77")
	a.RefreshLocale()

	require.Equal(t, "77", a.rows[0].input.Value())
}

// Backspace has to keep deleting text rather than closing the dialog, or the
// user cannot correct a mistyped number.
func TestAutoSummarizeBackspaceEditsText(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	// The field is seeded with the current threshold, so there is always
	// something to delete and backspace must never close the dialog.
	require.True(t, a.BackspaceDeletesText())

	a.rows[0].input.SetValue("")
	require.False(t, a.BackspaceDeletesText(),
		"with an empty field backspace should walk out instead")
}

func TestTruncateToWidth(t *testing.T) {
	t.Parallel()

	require.Equal(t, "abc", truncateToWidth("abc", 5))
	require.Equal(t, "ab…", truncateToWidth("abcdef", 3))
	require.Equal(t, "", truncateToWidth("abc", 0))
	require.Equal(t, "…", truncateToWidth("abc", 1))
	// Wide runes must not be split mid-character.
	require.Equal(t, "日本語…", truncateToWidth("日本語です", 4))
}
