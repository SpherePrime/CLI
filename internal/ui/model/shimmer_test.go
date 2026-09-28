package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func newShimmerTestUI(t *testing.T) *UI {
	t.Helper()
	u := newFrameTestUI(t)
	u.shimmerEnabled = true
	return u
}

func TestShimmerClockArmsOnceWhileTheLogoIsVisible(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)

	require.NotNil(t, u.ensureShimmerClock(), "a visible wordmark must start the clock")
	require.True(t, u.shimmerRunning)
	require.Nil(t, u.ensureShimmerClock(), "the clock must never be armed twice")
}

func TestShimmerClockStaysOffWhenDisabled(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.shimmerEnabled = false

	require.Nil(t, u.ensureShimmerClock(), "the option being off must keep the clock idle")
	require.Equal(t, 0, u.shimmerStep(), "the wordmark must hold still")
}

func TestShimmerTickAdvancesOneStepAndReArms(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)
	_ = u.ensureShimmerClock()
	gen := u.shimmerGen

	require.NotNil(t, u.handleShimmerTick(shimmerTickMsg{gen: gen}))
	require.Equal(t, 1, u.shimmerStep())
	require.True(t, u.shimmerRunning)
}

func TestShimmerStaleTickDoesNothing(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)
	_ = u.ensureShimmerClock()
	stale := shimmerTickMsg{gen: u.shimmerGen}

	// The watchdog arms a replacement clock after a lost tick.
	u.shimmerRunning = false
	u.shimmerArmedAt = u.shimmerArmedAt.Add(-2 * shimmerClockLostAfter)
	require.NotNil(t, u.ensureShimmerClock())
	require.NotEqual(t, stale.gen, u.shimmerGen)

	require.Nil(t, u.handleShimmerTick(stale), "a stale tick must not move the shimmer")
	require.Equal(t, 0, u.shimmerStep())
}

func TestShimmerStepWrapsAfterOneFullLoop(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)
	_ = u.ensureShimmerClock()

	for range styles.ShimmerFrames {
		require.NotNil(t, u.handleShimmerTick(shimmerTickMsg{gen: u.shimmerGen}))
	}
	require.Equal(t, 0, u.shimmerStep(), "the loop must return to its starting frame")
}

func TestShimmerTickDoesNotFreezeTheLogo(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)
	u.View()
	require.Equal(t, 1, u.frames.Len())

	// A scroll-only update at the same step must be served from the cache,
	// while the next shimmer step must render afresh: the step is part of the
	// frame key, so memoization cannot freeze the wordmark.
	u.beginFrameUpdate()
	u.markScrollOnly()
	u.View()
	require.Equal(t, 1, u.frames.hits)

	u.beginFrameUpdate()
	require.NotNil(t, u.handleShimmerTick(shimmerTickMsg{gen: u.shimmerGen}))
	view := u.View()
	require.Equal(t, 1, u.frames.hits, "a new shimmer step must not reuse the previous frame")
	require.NotEmpty(t, view.Content)
}

func TestHeaderWordmarkShimmersPerStep(t *testing.T) {
	t.Parallel()
	u := newShimmerTestUI(t)

	h := newHeader(u.com)
	first := h.wordmark(u.com.Styles, 0)
	later := h.wordmark(u.com.Styles, 5)
	require.NotEqual(t, first, later, "the wordmark must change color as the clock advances")
	require.Equal(t, ansi.Strip(first), ansi.Strip(later), "only the colors may change")

	// Cached frames must survive a step change without leaking across layouts.
	cached := h.compactFrames.Frame("compact|1|false", 5, func(p int) string {
		return h.wordmark(u.com.Styles, p)
	})
	require.Equal(t, later, cached)
}
