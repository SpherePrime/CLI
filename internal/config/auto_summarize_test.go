package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func ptrInt(v int) *int       { return &v }
func ptrInt64(v int64) *int64 { return &v }

func TestGetAutoSummarizePolicyDefaults(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string]*Options{
		"nil options": nil,
		"empty":      {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := opts.GetAutoSummarizePolicy()
			require.Equal(t, DefaultAutoSummarizePercent, got.Percent)
			require.Equal(t, DefaultAutoSummarizeLargePercent, got.LargePercent)
			require.Equal(t, int64(DefaultAutoSummarizeLargeWindow), got.LargeWindow)
		})
	}
}

func TestGetAutoSummarizePolicyUsesConfiguredValues(t *testing.T) {
	t.Parallel()

	opts := &Options{
		AutoSummarizePercent:      ptrInt(70),
		AutoSummarizeLargePercent: ptrInt(90),
		AutoSummarizeLargeWindow:  ptrInt64(1_000_000),
	}

	got := opts.GetAutoSummarizePolicy()
	require.Equal(t, 70, got.Percent)
	require.Equal(t, 90, got.LargePercent)
	require.Equal(t, int64(1_000_000), got.LargeWindow)
}

// Zero means "unset" in the config file, so it must resolve to the default
// rather than clamping to the minimum.
func TestGetAutoSummarizePolicyTreatsZeroAsUnset(t *testing.T) {
	t.Parallel()

	opts := &Options{
		AutoSummarizePercent:      ptrInt(0),
		AutoSummarizeLargePercent: ptrInt(0),
		AutoSummarizeLargeWindow:  ptrInt64(0),
	}

	got := opts.GetAutoSummarizePolicy()
	require.Equal(t, DefaultAutoSummarizePercent, got.Percent)
	require.Equal(t, DefaultAutoSummarizeLargePercent, got.LargePercent)
	require.Equal(t, int64(DefaultAutoSummarizeLargeWindow), got.LargeWindow)
}

func TestGetAutoSummarizePolicyClampsPercentages(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   int
		want int
	}{
		{"below the floor", 10, 50},
		{"exactly the floor", 50, 50},
		{"above the ceiling", 100, 99},
		{"exactly the ceiling", 99, 99},
		{"negative", -5, 50},
		{"in range", 88, 88},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := (&Options{
				AutoSummarizePercent:      ptrInt(tc.in),
				AutoSummarizeLargePercent: ptrInt(tc.in),
			}).GetAutoSummarizePolicy()
			require.Equal(t, tc.want, opts.Percent)
			require.Equal(t, tc.want, opts.LargePercent)
		})
	}
}

// The threshold is expressed as a percentage of the window, so a small window
// and a large one compact at the same fill ratio instead of at ratios that
// drift apart with size.
func TestSummarizeAtUsesPercentPerWindow(t *testing.T) {
	t.Parallel()

	policy := AutoSummarizePolicy{
		Percent:      85,
		LargePercent: 95,
		LargeWindow:  500_000,
	}

	for _, tc := range []struct {
		cw   int64
		want int
	}{
		{32_000, 85},
		{128_000, 85},
		{200_000, 85},
		{499_999, 85},
		{500_000, 95},
		{1_000_000, 95},
		{2_000_000, 95},
	} {
		require.Equal(t, tc.want, policy.SummarizeAt(tc.cw), "window %d", tc.cw)
	}
}

// An unknown window has to disable compaction. Custom and local models often
// declare none, and guessing would truncate them on the first step.
func TestSummarizeAtUnknownWindow(t *testing.T) {
	t.Parallel()

	policy := AutoSummarizePolicy{Percent: 85, LargePercent: 95, LargeWindow: 500_000}
	require.Equal(t, 0, policy.SummarizeAt(0))
	require.Equal(t, 0, policy.SummarizeAt(-1))
}

// A configured breakpoint has to be honoured even when it is unusual.
func TestSummarizeAtRespectsCustomBreakpoint(t *testing.T) {
	t.Parallel()

	policy := AutoSummarizePolicy{Percent: 70, LargePercent: 99, LargeWindow: 1_000_000}
	require.Equal(t, 70, policy.SummarizeAt(999_999))
	require.Equal(t, 99, policy.SummarizeAt(1_000_000))
}

// A non-positive breakpoint must not make every window count as large.
func TestSummarizeAtZeroBreakpointFallsBackToPercent(t *testing.T) {
	t.Parallel()

	policy := AutoSummarizePolicy{Percent: 85, LargePercent: 95, LargeWindow: 0}
	require.Equal(t, 85, policy.SummarizeAt(128_000))
	require.Equal(t, 85, policy.SummarizeAt(1_000_000))
}

func TestDefaultAutoSummarizePolicyMatchesUnset(t *testing.T) {
	t.Parallel()

	require.Equal(t, (&Options{}).GetAutoSummarizePolicy(), DefaultAutoSummarizePolicy())
}
