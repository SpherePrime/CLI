package model

import (
	"fmt"
	"image"
	"strings"
	"testing"
)

func layoutAreas(l uiLayout) map[string]image.Rectangle {
	return map[string]image.Rectangle{
		"area":           l.area,
		"header":         l.header,
		"main":           l.main,
		"pills":          l.pills,
		"editor":         l.editor,
		"sidebar":        l.sidebar,
		"status":         l.status,
		"sessionDetails": l.sessionDetails,
	}
}

func layoutTestUI(compact bool, prompt string) *UI {
	u := newTestUI()
	u.forceCompactMode = compact
	u.isCompact = compact
	u.textarea.SetValue(prompt)
	return u
}

func TestGenerateLayoutRectsAreNeverInverted(t *testing.T) {
	t.Parallel()

	widths := []int{1, 2, 20, 40, 60, 79, 80, 119, 120, 121, 160, 200, 250}
	heights := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 15, 20, 25, 29, 30, 31, 40, 45, 60, 80}
	longPrompt := strings.Repeat("wrapping prompt text ", 40)

	for _, compact := range []bool{false, true} {
		for _, prompt := range []string{"", "short prompt", longPrompt} {
			for _, width := range widths {
				for _, height := range heights {
					name := fmt.Sprintf("compact=%v/prompt=%d/w=%d/h=%d", compact, len(prompt), width, height)

					u := layoutTestUI(compact, prompt)
					l := u.generateLayout(width, height)

					for areaName, rect := range layoutAreas(l) {
						if rect.Dx() < 0 || rect.Dy() < 0 {
							t.Errorf("%s: %s rect %v has negative size", name, areaName, rect)
							continue
						}
						if rect.Min.X < 0 || rect.Min.Y < 0 {
							t.Errorf("%s: %s rect %v starts outside the screen", name, areaName, rect)
						}
						if rect.Max.X > width || rect.Max.Y > height {
							t.Errorf("%s: %s rect %v overflows the %dx%d screen", name, areaName, rect, width, height)
						}
					}
				}
			}
		}
	}
}

func TestGenerateLayoutKeepsChatVisibleOnShortWindows(t *testing.T) {
	t.Parallel()

	longPrompt := strings.Repeat("wrapping prompt text ", 40)

	for _, compact := range []bool{false, true} {
		for _, prompt := range []string{"", "short prompt", longPrompt} {
			for _, height := range []int{6, 7, 8, 10, 12, 20, 29, 30, 45} {
				width := 200

				u := layoutTestUI(compact, prompt)
				l := u.generateLayout(width, height)

				if l.main.Dy() < 1 {
					t.Errorf("compact=%v prompt=%d h=%d: expected chat to keep at least one row, got main %v",
						compact, len(prompt), height, l.main)
				}
				if l.editor.Dy()+l.main.Dy() > l.area.Dy() {
					t.Errorf("compact=%v prompt=%d h=%d: editor %v and main %v do not fit in %v",
						compact, len(prompt), height, l.editor, l.main, l.area)
				}
			}
		}
	}
}

func TestUpdateSizeKeepsComponentsUsableOnTinyWindows(t *testing.T) {
	t.Parallel()

	for _, width := range []int{1, 5, 20, 40} {
		for _, height := range []int{1, 3, 5, 8} {
			u := layoutTestUI(true, strings.Repeat("x", 200))
			u.width = width
			u.height = height
			u.updateLayoutAndSize()

			if u.layout.main.Dx() < 0 || u.layout.main.Dy() < 0 {
				t.Errorf("w=%d h=%d: main rect %v is negative", width, height, u.layout.main)
			}
			if got := u.chat.Height(); got < 0 {
				t.Errorf("w=%d h=%d: chat height %d is negative", width, height, got)
			}
		}
	}
}
