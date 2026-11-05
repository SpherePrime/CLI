package agent

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCompactTerminalOutput(t *testing.T) {
	raw := "\x1b[32mok\x1b[0m   \r\nprogress 10%\rprogress 100%\n\n\n\nline\nline\nline\nline\nend"
	require.Equal(t, "ok\nprogress 100%\n\nline\n[previous line repeated 3 more times]\nend", compactToolOutput("bash", raw))
}

func TestCompactKeepsShortRepeats(t *testing.T) {
	require.Equal(t, "a\na\nb", compactToolOutput("bash", "a\na\nb"))
}

func TestCompactSkipsFileContent(t *testing.T) {
	raw := "1|x  \n2|x  \n"
	require.Equal(t, raw, compactToolOutput("view", raw))
}

func TestCompactMCPJSON(t *testing.T) {
	raw := "{\n  \"items\": [\n    1,\n    2\n  ]\n}"
	require.Equal(t, `{"items":[1,2]}`, compactToolOutput("mcp_docs_search", raw))
	require.Equal(t, "plain\ntext", compactToolOutput("mcp_docs_search", "plain   \ntext"))
}

func TestCompactLeavesSingleLineJSON(t *testing.T) {
	raw := `{"a": 1}`
	require.Equal(t, raw, compactToolOutput("mcp_x_y", raw))
	require.False(t, strings.Contains(compactToolOutput("bash", "\x1b]0;title\x07done"), "title"))
}
