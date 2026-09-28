package model

import (
	"time"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

const (
	// shimmerFPS is how often the iridescent spectrum is moved one step along
	// the wordmark. It is deliberately slower than the spinners: the shimmer
	// is a slow sheen, and a lower rate keeps idle redraws cheap.
	shimmerFPS = 12

	// shimmerClockLostAfter is how long an armed shimmer clock may go without
	// its tick arriving before it is armed again.
	shimmerClockLostAfter = 2 * time.Second
)

// shimmerTickMsg is the clock that walks the iridescent ramp across the
// wordmark and other brand surfaces. It is separate from the chat's spinner
// clock because the logo shimmers while the agent is idle too.
type shimmerTickMsg struct{ gen uint64 }

// shimmerInterval is how long each shimmer frame stays on screen.
func shimmerInterval() time.Duration {
	return time.Second / time.Duration(shimmerFPS)
}

// shimmerActive reports whether the shimmer should be moving: the theme has a
// ramp to walk, the option is on, and a surface that uses it is on screen.
func (m *UI) shimmerActive() bool {
	if !m.shimmerEnabled || len(m.com.Styles.Iridescence) == 0 {
		return false
	}
	switch m.state {
	case uiOnboarding, uiInitialize, uiLanding, uiChat:
		return true
	default:
		return false
	}
}

// shimmerStep is the frame the brand surfaces should render at. With the
// shimmer off it stays put, so the wordmark keeps its iridescent gradient
// without moving.
func (m *UI) shimmerStep() int {
	if !m.shimmerEnabled {
		return 0
	}
	return m.shimmerFrame % styles.ShimmerFrames
}

// ensureShimmerClock arms the shimmer clock when a shimmering surface is on
// screen and no tick is outstanding. Like the spinner clock it is armed only
// from the tail of Update, so a tick never sits inside a caller's sequence.
func (m *UI) ensureShimmerClock() tea.Cmd {
	if !m.shimmerActive() {
		m.shimmerRunning = false
		return nil
	}
	if m.shimmerRunning && time.Since(m.shimmerArmedAt) < shimmerClockLostAfter {
		return nil
	}
	return m.armShimmerClock()
}

func (m *UI) armShimmerClock() tea.Cmd {
	m.shimmerRunning = true
	m.shimmerArmedAt = time.Now()
	m.shimmerGen++
	gen := m.shimmerGen
	return tea.Tick(shimmerInterval(), func(time.Time) tea.Msg {
		return shimmerTickMsg{gen: gen}
	})
}

// handleShimmerTick advances the shared step by one frame and re-arms the
// clock. Each step's logo is rendered once and then reused, so a tick costs a
// redraw rather than a re-render of the wordmark. The shimmer step is part of
// the frame cache key, so serving memoized frames while shimmering is safe.
func (m *UI) handleShimmerTick(msg shimmerTickMsg) tea.Cmd {
	if msg.gen != m.shimmerGen {
		return nil
	}
	m.shimmerRunning = false
	if !m.shimmerActive() {
		return nil
	}
	m.shimmerFrame = (m.shimmerFrame + 1) % styles.ShimmerFrames
	m.tintTodoSpinner()
	m.markScrollOnly()
	return m.armShimmerClock()
}

// tintTodoSpinner walks the to-do pill's spinner along the same ramp as the
// wordmark, so the pill matches the rest of the brand surfaces instead of
// sitting in one hue.
func (m *UI) tintTodoSpinner() {
	ramp := m.com.Styles.IridescentRamp
	if len(ramp) == 0 {
		return
	}
	tint := styles.ShimmerColor(ramp, styles.ShiftShimmer(0, m.shimmerFrame))
	m.todoSpinner.Style = m.com.Styles.Pills.TodoSpinner.Foreground(tint)
}
