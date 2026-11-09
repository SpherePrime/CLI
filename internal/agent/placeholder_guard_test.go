package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type recordingTool struct {
	fantasy.AgentTool
	calls int
}

func (r *recordingTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	r.calls++
	return fantasy.NewTextResponse("ok"), nil
}

func TestPlaceholderGuardRejectsCopiedPlaceholders(t *testing.T) {
	inner := &recordingTool{}
	guarded := wrapToolsWithPlaceholderGuard([]fantasy.AgentTool{inner})[0]
	for _, input := range []string{
		`{"file_path":"src/app.js","content":"[15808 bytes omitted from an earlier call]"}`,
		`{"file_path":"src/app.js","content":"` + agedPlaceholder(15808) + `"}`,
	} {
		response, err := guarded.Run(context.Background(), fantasy.ToolCall{Name: "write", Input: input})
		require.NoError(t, err)
		require.True(t, response.IsError)
		require.Contains(t, response.Content, "placeholder")
	}
	require.Zero(t, inner.calls)

	response, err := guarded.Run(context.Background(), fantasy.ToolCall{Name: "write", Input: `{"file_path":"a","content":"real"}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, 1, inner.calls)
}
