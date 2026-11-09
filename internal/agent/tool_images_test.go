package agent

import (
	"encoding/base64"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func encodedImage(marker string) string {
	return base64.StdEncoding.EncodeToString([]byte("png-" + marker))
}

func createRunWithImages(t *testing.T, env fakeEnv, sessionID, callID string, count int) {
	t.Helper()
	ctx := t.Context()
	_, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: callID, Name: "mcp_browser_browser_run", Input: `{}`, Finished: true},
		},
	})
	require.NoError(t, err)
	images := make([]message.ToolResultImage, 0, count)
	for index := range count {
		images = append(images, message.ToolResultImage{Data: encodedImage(callID + string(rune('a'+index))), MIMEType: "image/png"})
	}
	_, err = env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: callID, Name: "mcp_browser_browser_run", Content: "Scenario PASSED", Data: images[0].Data, MIMEType: "image/png", Images: images},
		},
	})
	require.NoError(t, err)
}

func imageMessagesAfterTool(history []fantasy.Message) [][]fantasy.FilePart {
	var groups [][]fantasy.FilePart
	for index, entry := range history {
		if entry.Role != fantasy.MessageRoleUser || index == 0 || history[index-1].Role != fantasy.MessageRoleTool {
			continue
		}
		var files []fantasy.FilePart
		for _, part := range entry.Content {
			if file, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
				files = append(files, file)
			}
		}
		groups = append(groups, files)
	}
	return groups
}

func TestPreparePromptSendsEveryToolImageToVisionModels(t *testing.T) {
	env := testEnv(t)
	agent := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "check the site"}}})
	require.NoError(t, err)
	for _, callID := range []string{"run_1", "run_2", "run_3"} {
		createRunWithImages(t, env, sess.ID, callID, 3)
	}
	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, true)
	groups := imageMessagesAfterTool(history)
	require.Len(t, groups, 2)
	for groupIndex, callID := range []string{"run_2", "run_3"} {
		require.Len(t, groups[groupIndex], 2)
		require.Equal(t, []byte("png-"+callID+"b"), groups[groupIndex][0].Data)
		require.Equal(t, []byte("png-"+callID+"c"), groups[groupIndex][1].Data)
	}

	textOnly, _ := agent.preparePrompt(msgs, false)
	require.Empty(t, imageMessagesAfterTool(textOnly))
}
