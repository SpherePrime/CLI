package agent

import (
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
