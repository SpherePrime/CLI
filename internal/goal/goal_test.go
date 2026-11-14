package goal

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStorePersistsGoalAcrossReconnect(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := db.Connect(ctx, dir)
	require.NoError(t, err)
	sessions := session.NewService(db.New(connection), connection)
	chat, err := sessions.Create(ctx, "chat")
	require.NoError(t, err)

	store := NewStore(connection)
	_, err = store.Set(ctx, chat.ID, "   ")
	require.ErrorIs(t, err, ErrEmptyText)
	saved, err := store.Set(ctx, chat.ID, " build the app ")
	require.NoError(t, err)
	require.Equal(t, "build the app", saved.Text)
	require.True(t, saved.Active())

	require.NoError(t, db.Release(dir))
	connection, err = db.Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	store = NewStore(connection)

	loaded, found, err := store.Get(ctx, chat.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "build the app", loaded.Text)

	achieved, err := store.Complete(ctx, chat.ID, "done")
	require.NoError(t, err)
	require.Equal(t, StatusAchieved, achieved.Status)
	_, err = store.Complete(ctx, chat.ID, "again")
	require.ErrorIs(t, err, ErrNoActiveGoal)

	reset, err := store.Set(ctx, chat.ID, "next goal")
	require.NoError(t, err)
	require.True(t, reset.Active())
	require.Empty(t, reset.Summary)

	require.NoError(t, store.Clear(ctx, chat.ID))
	_, found, err = store.Get(ctx, chat.ID)
	require.NoError(t, err)
	require.False(t, found)
}

func todos(statuses ...session.TodoStatus) []session.Todo {
	items := make([]session.Todo, len(statuses))
	for index, status := range statuses {
		items[index] = session.Todo{Content: string(rune('a' + index)), Status: status}
	}
	return items
}

var activeGoal = Goal{Text: "ship it", Status: StatusActive}

func TestDriverContinuesWhileStagesRemain(t *testing.T) {
	driver := NewDriver()
	prompt, reason := driver.Next(activeGoal, true, todos(session.TodoStatusCompleted, session.TodoStatusInProgress, session.TodoStatusPending))
	require.Equal(t, StopNone, reason)
	require.Contains(t, prompt, "Next: b")

	prompt, reason = driver.Next(activeGoal, true, todos(session.TodoStatusCompleted, session.TodoStatusCompleted))
	require.Equal(t, StopNone, reason)
	require.Contains(t, prompt, CompleteToolName)
	require.Contains(t, prompt, "Final acceptance")
	require.Contains(t, prompt, "browser_*")
	require.Contains(t, prompt, "VERDICT: APPROVED")
	require.Contains(t, prompt, "do NOT need to be re-tested or re-checked from scratch")
}

func TestDriverStopsWithoutGoalPlanOrAfterAchievement(t *testing.T) {
	pending := todos(session.TodoStatusPending)
	_, reason := NewDriver().Next(Goal{}, false, pending)
	require.Equal(t, StopNoGoal, reason)
	_, reason = NewDriver().Next(Goal{Text: "x", Status: StatusAchieved}, true, pending)
	require.Equal(t, StopAchieved, reason)
	_, reason = NewDriver().Next(activeGoal, true, nil)
	require.Equal(t, StopNoPlan, reason)
}

func TestDriverStopsWhenStagesStopChanging(t *testing.T) {
	driver := NewDriver()
	stuck := todos(session.TodoStatusInProgress, session.TodoStatusPending)
	for range DefaultStallLimit {
		_, reason := driver.Next(activeGoal, true, stuck)
		require.Equal(t, StopNone, reason)
	}
	_, reason := driver.Next(activeGoal, true, stuck)
	require.Equal(t, StopStalled, reason)

	_, reason = driver.Next(activeGoal, true, todos(session.TodoStatusCompleted, session.TodoStatusInProgress))
	require.Equal(t, StopNone, reason)
}

func TestDriverHonoursStepLimit(t *testing.T) {
	driver := &Driver{StallLimit: 100, MaxSteps: 2}
	_, reason := driver.Next(activeGoal, true, todos(session.TodoStatusPending))
	require.Equal(t, StopNone, reason)
	_, reason = driver.Next(activeGoal, true, todos(session.TodoStatusInProgress))
	require.Equal(t, StopNone, reason)
	_, reason = driver.Next(activeGoal, true, todos(session.TodoStatusCompleted))
	require.Equal(t, StopStepLimit, reason)
}

func TestNoteAsksQuestionsBeforePlanningAndResumesAfter(t *testing.T) {
	fresh := Note(activeGoal, Progress{})
	require.Contains(t, fresh, "question tool")
	require.Contains(t, fresh, "Final acceptance of the whole product")
	require.Contains(t, fresh, "ship it")
	resumed := Note(activeGoal, ProgressOf(todos(session.TodoStatusCompleted, session.TodoStatusPending)))
	require.Contains(t, resumed, "1 of 2")
	require.Contains(t, resumed, "Resume from this point")
	require.Contains(t, resumed, "without re-checking already APPROVED unmodified parts")
}
