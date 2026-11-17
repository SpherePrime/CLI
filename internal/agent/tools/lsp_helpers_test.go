package tools

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestGetSymbolOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		symbol string
		want   int
	}{
		{"bare symbol", "Bar", 0},
		{"dot qualified", "foo.Bar", 4},
		{"double colon qualified", "Class::method", 7},
		{"backslash qualified", `ns\Func`, 3},
		{"nested dots", "a.b.C", 4},
		{"empty", "", 0},
		{"single char", "x", 0},
		{"dot at end", "foo.", 4},
		{"colon at end", "foo::", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getSymbolOffset(tt.symbol)
			require.Equal(t, tt.want, got, "getSymbolOffset(%q)", tt.symbol)
		})
	}
}

// TestGetSymbolOffset_DoesNotOvershoot verifies that the offset lands
// on the start of the final component, never past it.
func TestGetSymbolOffset_DoesNotOvershoot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		symbol   string
		expected string // the substring starting at the offset
	}{
		{"foo.Bar", "Bar"},
		{"Class::method", "method"},
		{`ns\Func`, "Func"},
		{"a.b.c.D", "D"},
		{"Bar", "Bar"},
	}

	for _, tc := range cases {
		offset := getSymbolOffset(tc.symbol)
		require.LessOrEqual(t, offset, len(tc.symbol),
			"offset %d exceeds symbol length %d for %q", offset, len(tc.symbol), tc.symbol)
		got := tc.symbol[offset:]
		require.Equal(t, tc.expected, got,
			"getSymbolOffset(%q) = %d, remainder = %q, want %q",
			tc.symbol, offset, got, tc.expected)
	}
}

func TestSymbolSearchInclude(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", symbolSearchInclude(nil))
	require.Equal(t, "", symbolSearchInclude(&lsp.Manager{}))
}

func TestIncludeGlobFromFileTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lists [][]string
		want  string
	}{
		{
			name:  "no clients",
			lists: nil,
			want:  "",
		},
		{
			name:  "wildcard client disables filter",
			lists: [][]string{{"go"}, {}},
			want:  "",
		},
		{
			name:  "single extension",
			lists: [][]string{{".go"}},
			want:  "*.go",
		},
		{
			name:  "go language",
			lists: [][]string{{"go"}},
			want:  "*.go",
		},
		{
			name:  "rust language",
			lists: [][]string{{"rust"}},
			want:  "*.rs",
		},
		{
			name:  "python language",
			lists: [][]string{{"python"}},
			want:  "*.{py,pyi}",
		},
		{
			name:  "multiple clients sorted deterministically",
			lists: [][]string{{"typescript", "typescriptreact"}, {"go", ".go"}},
			want:  "*.{cts,go,mts,ts,tsx}",
		},
		{
			name:  "extensionless filetype disables filter",
			lists: [][]string{{"go"}, {"dockerfile"}},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, includeGlobFromFileTypes(tt.lists))
		})
	}
}
