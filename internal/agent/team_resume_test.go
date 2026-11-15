package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
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

	roster := teamRosterNote([]AgentWorker{{ID: "aj-2", Name: "ui", Agent: config.AgentCode, Status: "done", Reviewer: "ui-review", Task: "build ui"}})
	rosterMessages := withTeamNote([]fantasy.Message{fantasy.NewUserMessage("add toggle")}, roster)
	rosterMessages = withTeamNote(rosterMessages, roster)
	require.Len(t, rosterMessages, 2)
	rosterText, _ := fantasy.AsMessagePart[fantasy.TextPart](rosterMessages[0].Content[0])
	require.True(t, strings.HasPrefix(rosterText.Text, teamRosterHeader))
}

func TestTeamRosterNoteListsAllWorkersAndReuseRules(t *testing.T) {
	require.Empty(t, teamRosterNote(nil))
	note := teamRosterNote([]AgentWorker{
		{ID: "w-1", Name: "repo-scan", Agent: config.AgentExplore, Status: "done", Task: "inspect auth package"},
		{ID: "w-2", Name: "auth-code", Agent: config.AgentCode, Status: "idle", Reviewer: "auth-review", Task: "implement login"},
		{ID: "w-3", Name: "auth-review", Agent: config.AgentReview, Status: "canceled", Task: "review login"},
	})
	require.Contains(t, note, teamRosterHeader)
	require.Contains(t, note, "repo-scan (worker_id w-1, role explore, status done): inspect auth package")
	require.Contains(t, note, "auth-code (worker_id w-2, role code, status idle, reviewer auth-review): implement login")
	require.Contains(t, note, "auth-review (worker_id w-3, role review, status canceled): review login")
	require.Contains(t, note, "agent_message (op=send, worker_id=<worker_id>)")
	require.Contains(t, note, "Do NOT spawn new -v2 or -fix agents")
}

func TestCoordinatorTeamNoteIncludesDoneAndIdleWithReviewer(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	parentMsg, err := env.messages.Create(t.Context(), parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call-hist",
				Name:     AgentToolName,
				Input:    `{"name":"hist-ui","agent":"code","prompt":"build footer","reviewer":"hist-review"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	histID := env.sessions.CreateAgentToolSessionID(parentMsg.ID, "call-hist")
	_, err = env.sessions.CreateTaskSession(t.Context(), histID, parent.ID, "Agent Session: code | hist-ui")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), histID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "build footer"}},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), histID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "footer built"}},
	})
	require.NoError(t, err)

	liveCode := newMockAgent("job-provider", 1024, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return agentResultWithText("api done"), nil
	})
	liveJob, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          liveCode,
		SessionID:      parent.ID,
		AgentMessageID: "m-live",
		ToolCallID:     "call-live",
		Prompt:         "build api",
		SessionTitle:   "Agent Session: code | live-api",
		Reviewer:       "api-review",
	}, config.AgentCode)
	require.NoError(t, err)
	require.True(t, liveJob.wait(t.Context(), 2*time.Second))

	note := c.teamNote(t.Context(), parent.ID)
	require.Contains(t, note, "hist-ui (worker_id "+histID+", role code, status idle, reviewer hist-review): build footer")
	require.Contains(t, note, "live-api (worker_id "+liveJob.id+", role code, status done, reviewer api-review): build api")
	require.Empty(t, c.teamNote(t.Context(), liveJob.sessionID))

	c.cfg.Config().Options.DisableMultiAgent = true
	require.Empty(t, c.teamNote(t.Context(), parent.ID))
}
