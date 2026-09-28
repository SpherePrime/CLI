package logo

import "github.com/SpherePrime/CLI/internal/ui/styles"

// Frames caches the shimmer frames of one logo layout, rendering each frame
// only the first time it is asked for. The animation clock advances a step per
// tick; without this cache every tick would repaint and re-encode the whole
// wordmark.
type Frames struct {
	key    string
	frames []string
}

// Frame returns the logo as it should look at the given shimmer step,
// building it with render on the first request. A new key marks the cached
// frames stale: the logo changed size, layout, or theme.
func (f *Frames) Frame(key string, step int, render func(phase int) string) string {
	if f == nil || render == nil {
		return ""
	}
	if key != f.key || len(f.frames) != styles.ShimmerFrames {
		f.key = key
		f.frames = make([]string, styles.ShimmerFrames)
	}
	i := step % styles.ShimmerFrames
	if i < 0 {
		i += styles.ShimmerFrames
	}
	if f.frames[i] == "" {
		f.frames[i] = render(i)
	}
	return f.frames[i]
}
