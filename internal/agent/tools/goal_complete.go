package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/SpherePrime/CLI/internal/goal"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

//go:embed goal_complete.md
var goalCompleteDescription string

const GoalCompleteToolName = goal.CompleteToolName

type GoalCompleteParams struct {
	Summary string `json:"summary" description:"What was delivered and how it was verified"`
}

func NewGoalCompleteTool(goals goal.Store, sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		GoalCompleteToolName,
		goalCompleteDescription,
		func(ctx context.Context, params GoalCompleteParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required to complete a goal")
			}
			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}
			if progress := goal.ProgressOf(currentSession.Todos); progress.Total > progress.Completed {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%d of %d stages are still open (next: %s). Finish and verify them, or remove stages that are no longer needed, before completing the goal.", progress.Total-progress.Completed, progress.Total, progress.Next)), nil
			}
			if _, err := goals.Complete(ctx, sessionID, params.Summary); err != nil {
				if errors.Is(err, goal.ErrNoActiveGoal) {
					return fantasy.NewTextErrorResponse("This chat has no active goal."), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("failed to complete goal: %w", err)
			}
			return fantasy.NewTextResponse("Goal marked as achieved. Give the user a short final summary."), nil
		},
	)
}
