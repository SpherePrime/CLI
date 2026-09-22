package common

import (
	"strings"

	"github.com/dwertyfa288/CLI/internal/ui/styles"
)

// Scrollbar renders a vertical scrollbar based on content and viewport size.
// Returns an empty string if content fits within viewport (no scrolling needed).
func Scrollbar(s *styles.Styles, height, contentSize, viewportSize, offset int) string {
	thumbPos, thumbSize, ok := ScrollbarGeometry(height, contentSize, viewportSize, offset)
	if !ok {
		return ""
	}

	// Build the scrollbar.
	var sb strings.Builder
	for i := range height {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(s.Dialog.ScrollbarThumb.Render(styles.ScrollbarThumb))
		} else {
			sb.WriteString(s.Dialog.ScrollbarTrack.Render(styles.ScrollbarTrack))
		}
	}

	return sb.String()
}
