package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/goal"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func runGoalComplete(t *testing.T, tool fantasy.AgentTool, sessionID string) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(GoalCompleteParams{Summary: "shipped"})
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sessionID)
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: GoalCompleteToolName, Input: string(input)})
	require.NoError(t, err)
	return response
}

func TestGoalCompleteRequiresClosedStages(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := db.Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	sessions := session.NewService(db.New(connection), connection)
	goals := goal.NewStore(connection)
	tool := NewGoalCompleteTool(goals, sessions)

	chat, err := sessions.Create(ctx, "chat")
	require.NoError(t, err)
	require.True(t, runGoalComplete(t, tool, chat.ID).IsError)

	_, err = goals.Set(ctx, chat.ID, "ship")
	require.NoError(t, err)
	chat.Todos = []session.Todo{{Content: "build", Status: session.TodoStatusCompleted}, {Content: "test", Status: session.TodoStatusPending}}
	chat, err = sessions.Save(ctx, chat)
	require.NoError(t, err)
	response := runGoalComplete(t, tool, chat.ID)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "test")

	chat.Todos[1].Status = session.TodoStatusCompleted
	_, err = sessions.Save(ctx, chat)
	require.NoError(t, err)
	require.False(t, runGoalComplete(t, tool, chat.ID).IsError)
	stored, _, err := goals.Get(ctx, chat.ID)
	require.NoError(t, err)
	require.Equal(t, goal.StatusAchieved, stored.Status)
	require.Equal(t, "shipped", stored.Summary)
}
