package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/goal"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWithGoalNoteGoesAfterSystemPromptOnce(t *testing.T) {
	note := goal.Note(goal.Goal{Text: "ship", Status: goal.StatusActive}, goal.Progress{})
	messages := []fantasy.Message{fantasy.NewSystemMessage("system"), fantasy.NewUserMessage("hi")}

	withNote := withGoalNote(messages, note)
	require.Len(t, withNote, 3)
	require.Equal(t, fantasy.MessageRoleSystem, withNote[1].Role)
	require.Equal(t, fantasy.MessageRoleUser, withNote[2].Role)

	require.Len(t, withGoalNote(withNote, note), 3)
	require.Len(t, withGoalNote(messages, ""), 2)
}

func TestWithoutGoalToolsHidesGoalComplete(t *testing.T) {
	newTool := func(name string) fantasy.AgentTool {
		return fantasy.NewAgentTool(name, name, func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
	}
	available := []fantasy.AgentTool{newTool("view"), newTool(goal.CompleteToolName), newTool("bash")}

	filtered := withoutGoalTools(available)

	require.Len(t, filtered, 2)
	require.Equal(t, "view", filtered[0].Info().Name)
	require.Equal(t, "bash", filtered[1].Info().Name)
	require.Len(t, available, 3)
	require.Equal(t, goal.CompleteToolName, available[1].Info().Name)
}
