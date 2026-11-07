package backend

import (
	"context"
	"errors"

	"github.com/SpherePrime/CLI/internal/goal"
)

var ErrGoalsUnavailable = errors.New("chat goals are not available")

func (b *Backend) workspaceGoals(workspaceID string) (goal.Store, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Goals == nil {
		return nil, ErrGoalsUnavailable
	}
	return ws.Goals, nil
}

func (b *Backend) GetSessionGoal(ctx context.Context, workspaceID, sessionID string) (goal.Goal, bool, error) {
	goals, err := b.workspaceGoals(workspaceID)
	if err != nil {
		return goal.Goal{}, false, err
	}
	return goals.Get(ctx, sessionID)
}

func (b *Backend) SetSessionGoal(ctx context.Context, workspaceID, sessionID, text string) (goal.Goal, error) {
	goals, err := b.workspaceGoals(workspaceID)
	if err != nil {
		return goal.Goal{}, err
	}
	return goals.Set(ctx, sessionID, text)
}

func (b *Backend) ClearSessionGoal(ctx context.Context, workspaceID, sessionID string) error {
	goals, err := b.workspaceGoals(workspaceID)
	if err != nil {
		return err
	}
	return goals.Clear(ctx, sessionID)
}
