package dialog

import "testing"

func TestFormatKeyCombo(t *testing.T) {
	cases := map[string]string{
		"":                    "unbound",
		"alt+v":               "Alt+V",
		"shift+enter":         "Shift+Enter",
		"ctrl+shift+space":    "Ctrl+Shift+Space",
		"shift+enter, ctrl+j": "Shift+Enter / Ctrl+J",
		"enter":               "Enter",
		"esc":                 "Esc",
		"ctrl+r+r":            "Ctrl+R+R",
		"←/→":                 "←/→",
	}
	for in, want := range cases {
		if got := formatKeyCombo(in, "unbound"); got != want {
			t.Errorf("formatKeyCombo(%q, %q) = %q, want %q", in, "unbound", got, want)
		}
	}
}

// The empty/whitespace cases must fall back to the localized unbound label,
// not a hard-coded English word: an unbound row in a Russian console has to
// read "не назначено", not "unbound".
func TestFormatKeyComboUsesUnboundLabel(t *testing.T) {
	for _, in := range []string{"", " , ", " ,  "} {
		if got := formatKeyCombo(in, "не назначено"); got != "не назначено" {
			t.Errorf("formatKeyCombo(%q, %q) = %q, want the localized label", in, "не назначено", got)
		}
	}
}
