package logo

import (
	"strings"
	"testing"

	"github.com/dwertyfa288/CLI/internal/ui/styles"
	"github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/dwertyfa288/CLI/vendordeps/lipgloss/v2"
	"github.com/dwertyfa288/CLI/vendordeps/stretchr/testify/require"
)

// letterformCells is the column budget every letterform gets. The diagonal
// field rows and the right-aligned version are measured against the finished
// word, so a letter that runs wider than its neighbors drags the whole block
// out of alignment.
const letterformCells = 5

func TestLetterformsShareOneGrid(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		form letterform
	}{
		{"C", LetterC},
		{"E", LetterE},
		{"EAlt", LetterEAlt},
		{"H", LetterH},
		{"I", LetterI},
		{"M", LetterM},
		{"P", LetterP},
		{"R", LetterR},
		{"SAlt", LetterSAlt},
		{"U", LetterU},
		{"Y", LetterY},
		{"YAlt", LetterYAlt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, letterformCells, lipgloss.Width(tt.form(false)),
				"unstretched %s must fill exactly %d cells", tt.name, letterformCells)
		})
	}
}

func TestRenderCompactFramesTheTitleSymmetrically(t *testing.T) {
	t.Parallel()

	const version = "v1.2.3"

	out := Render(testTheme().Logo.GradCanvas, version, true, testOpts())
	lines := strings.Split(out, "\n")

	// One field row, the version row, three title rows, one field row.
	require.Len(t, lines, 6)

	titleWidth := lipgloss.Width(lines[2])
	for i, line := range lines {
		require.Equal(t, titleWidth, lipgloss.Width(line), "row %d must match the title width", i)
	}

	top, bottom := ansi.Strip(lines[0]), ansi.Strip(lines[len(lines)-1])
	require.Equal(t, strings.Repeat(diag, titleWidth), top, "the field row must span the title")
	require.Equal(t, top, bottom, "the block needs the same framing above and below")

	meta := ansi.Strip(lines[1])
	require.True(t, strings.HasSuffix(meta, version),
		"the version should sit on the right edge, got %q", meta)
	require.False(t, strings.HasSuffix(out, "\n"), "the compact logo should not carry a trailing blank row")
}

func testTheme() styles.Styles { return styles.ThemeForProvider("") }

func testOpts() Opts {
	t := testTheme().Logo
	return Opts{
		FieldColor:   t.FieldColor,
		TitleColorA:  t.TitleColorA,
		TitleColorB:  t.TitleColorB,
		LabelColor:   t.LabelColor,
		VersionColor: t.VersionColor,
	}
}
