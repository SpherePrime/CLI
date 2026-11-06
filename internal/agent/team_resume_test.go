package agent

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSortWorkersKeepsStartOrderRegardlessOfStatus(t *testing.T) {
	workers := []AgentWorker{
		{ID: "c", SessionID: "s3", StartedAt: 20, Status: "running"},
		{ID: "b", SessionID: "s2", StartedAt: 10, Status: "idle"},
		{ID: "a", SessionID: "s1", StartedAt: 10, Status: "running"},
	}
	sortWorkers(workers)
	require.Equal(t, []string{"a", "b", "c"}, []string{workers[0].ID, workers[1].ID, workers[2].ID})
}

func TestStoppedWorkersNoteListsOnlyStoppedWorkers(t *testing.T) {
	note := stoppedWorkersNote([]AgentWorker{
		{ID: "aj-1", Name: "Scout", Agent: "task", Task: "find bugs", Status: "canceled"},
		{ID: "aj-2", Name: "Done", Agent: "task", Task: "finished", Status: "done"},
		{ID: "s-3", Name: "Old", Agent: "task", Task: "resume me", Status: "interrupted"},
	})
	require.Contains(t, note, "worker_id aj-1")
	require.Contains(t, note, "worker_id s-3")
	require.NotContains(t, note, "aj-2")
	require.Contains(t, note, "agent_message")
	require.Empty(t, stoppedWorkersNote([]AgentWorker{{ID: "x", Status: "done"}}))
}

func TestWithTeamNoteAddsNoteOnce(t *testing.T) {
	note := stoppedWorkersNote([]AgentWorker{{ID: "aj-1", Status: "canceled"}})
	messages := withTeamNote([]fantasy.Message{fantasy.NewUserMessage("continue")}, note)
	messages = withTeamNote(messages, note)
	require.Len(t, messages, 2)
	text, _ := fantasy.AsMessagePart[fantasy.TextPart](messages[0].Content[0])
	require.True(t, strings.HasPrefix(text.Text, stoppedWorkersHeader))
	require.Len(t, withTeamNote(messages[1:], ""), 1)
}
