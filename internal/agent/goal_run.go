package agent

import (
	"context"
	"log/slog"
	"strings"

	"github.com/SpherePrime/CLI/internal/goal"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func (c *coordinator) sessionGoal(ctx context.Context, sessionID string) (goal.Goal, bool) {
	if c.goals == nil {
		return goal.Goal{}, false
	}
	current, found, err := c.goals.Get(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to read chat goal", "session_id", sessionID, "error", err)
		return goal.Goal{}, false
	}
	return current, found
}

func (c *coordinator) goalNote(ctx context.Context, sessionID string) (string, bool) {
	current, found := c.sessionGoal(ctx, sessionID)
	if !found || !current.Active() {
		return "", false
	}
	currentSession, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		return goal.Note(current, goal.Progress{}), true
	}
	return goal.Note(current, goal.ProgressOf(currentSession.Todos)), true
}

func (c *coordinator) nextGoalStep(ctx context.Context, sessionID string, driver *goal.Driver) (string, goal.StopReason) {
	if c.QueuedPrompts(sessionID) > 0 {
		return "", goal.StopQueued
	}
	current, found := c.sessionGoal(ctx, sessionID)
	currentSession, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		return "", goal.StopNoPlan
	}
	return driver.Next(current, found, currentSession.Todos)
}

func withGoalNote(messages []fantasy.Message, note string) []fantasy.Message {
	if note == "" {
		return messages
	}
	insertAt := 0
	for index, entry := range messages {
		if entry.Role != fantasy.MessageRoleSystem {
			break
		}
		for _, part := range entry.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.HasPrefix(text.Text, goal.NoteHeader) {
				return messages
			}
		}
		insertAt = index + 1
	}
	result := make([]fantasy.Message, 0, len(messages)+1)
	result = append(result, messages[:insertAt]...)
	result = append(result, fantasy.NewSystemMessage(note))
	return append(result, messages[insertAt:]...)
}
