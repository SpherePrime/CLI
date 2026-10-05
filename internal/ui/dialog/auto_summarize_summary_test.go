package dialog

import (
	"strconv"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// The summary line is the one place the dialog says which threshold will
// actually apply to the model in use, so it is what a user watches while typing.
//
// It used to read the thresholds back out of the config instead of out of the
// fields. Editing 85 to 90 therefore left the line claiming 85: the one number
// in the dialog that shows whether an edit took effect did not update.
func TestAutoSummarizeSummaryTracksTheEditedValue(t *testing.T) {
	t.Parallel()

	// A window below the large threshold, so the first field decides the outcome.
	a := newAutoSummarizeTestDialogWithWindow(t, &config.Options{}, 200_000)

	require.Contains(t, a.summaryLine(), strconv.Itoa(config.DefaultAutoSummarizePercent))

	a.rows[0].input.SetValue("90")
	require.Contains(t, a.summaryLine(), "90%",
		"the summary must reflect the value in the field, not the saved one")
	require.NotContains(t, a.summaryLine(), "85%")
}

// The same has to hold for the large-window side, and for a change that flips
// which of the two percentages applies.
func TestAutoSummarizeSummaryFollowsTheLargeWindowFields(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	// Drop the large-window threshold so the model is treated as large.
	a.rows[2].input.SetValue("1000")
	a.rows[1].input.SetValue("97")

	policy := a.editedPolicy()
	require.Equal(t, 97, policy.LargePercent)
	require.EqualValues(t, 1000, policy.LargeWindow)
}

// A value below the minimum is not a half-typed percentage, it is out of range,
// so it is clamped to the floor rather than treated as 9. This is the same rule
// the save path uses, which is why the summary cannot promise a number the save
// would then reject.
func TestAutoSummarizeSummaryClampsAnOutOfRangeEntry(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialogWithWindow(t, &config.Options{}, 200_000)
	a.rows[0].input.SetValue("9")

	require.Contains(t, a.summaryLine(), "50%",
		"an entry below the floor resolves to the floor")

	a.adjust(1)
	require.Equal(t, "51", a.rows[0].input.Value(),
		"the increment steps from the clamped value, not from the rejected text")
}

func TestAutoSummarizeEditedPolicyFallsBackPerField(t *testing.T) {
	t.Parallel()

	percent := 70
	a := newAutoSummarizeTestDialog(t, &config.Options{AutoSummarizePercent: &percent})

	// Valid in the first field, junk in the second and third.
	a.rows[0].input.SetValue("80")
	a.rows[1].input.SetValue("")
	a.rows[2].input.SetValue("not-a-number")

	policy := a.editedPolicy()
	require.Equal(t, 80, policy.Percent)
	require.Equal(t, config.DefaultAutoSummarizeLargePercent, policy.LargePercent,
		"an empty field keeps the saved value rather than becoming zero")
	require.EqualValues(t, config.DefaultAutoSummarizeLargeWindow, policy.LargeWindow)
}

// An out-of-range value is clamped by the same rule the fields use, so the
// summary cannot promise a threshold the save would reject.
func TestAutoSummarizeEditedPolicyClampsOutOfRange(t *testing.T) {
	t.Parallel()

	a := newAutoSummarizeTestDialog(t, &config.Options{})

	a.rows[0].input.SetValue("999")
	require.Equal(t, 99, a.editedPolicy().Percent)

	a.rows[0].input.SetValue("1")
	require.Equal(t, 50, a.editedPolicy().Percent)
}