package message

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMessagePagesStableCursorAndSessionIsolation(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	queries := db.New(conn)
	sessions := session.NewService(queries, conn)
	first, err := sessions.Create(t.Context(), "first")
	require.NoError(t, err)
	second, err := sessions.Create(t.Context(), "second")
	require.NoError(t, err)
	svc := NewService(queries)
	ids := make([]string, 7)
	for i := range ids {
		msg, err := svc.Create(t.Context(), first.ID, CreateMessageParams{Role: User})
		require.NoError(t, err)
		ids[i] = msg.ID
	}
	_, err = conn.ExecContext(t.Context(), "UPDATE messages SET created_at = 100 WHERE session_id = ?", first.ID)
	require.NoError(t, err)
	foreign, err := svc.Create(t.Context(), second.ID, CreateMessageParams{Role: User})
	require.NoError(t, err)
	pager := svc.(PagedService)
	var received []string
	cursor := ""
	for {
		page, err := pager.ListPage(t.Context(), first.ID, cursor, 3)
		require.NoError(t, err)
		pageIDs := make([]string, len(page.Messages))
		for i, msg := range page.Messages {
			pageIDs[i] = msg.ID
		}
		received = append(pageIDs, received...)
		if !page.HasMore {
			break
		}
		require.NotEmpty(t, page.Before)
		cursor = page.Before
	}
	require.Equal(t, ids, received)
	_, err = pager.ListPage(t.Context(), first.ID, foreign.ID, 3)
	require.ErrorIs(t, err, ErrInvalidCursor)
	_, err = pager.ListPage(t.Context(), first.ID, "missing", 3)
	require.ErrorIs(t, err, ErrInvalidCursor)
}

func TestMessagePagePayloadBudgetAndTextExport(t *testing.T) {
	svc, sessionID := newTestService(t)
	ids := make([]string, 3)
	for i := range ids {
		msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "visible"}, TextContent{Text: "hidden", Hidden: true}, BinaryContent{Data: []byte(strings.Repeat("x", 4*1024*1024)), MIMEType: "image/png"}}})
		require.NoError(t, err)
		ids[i] = msg.ID
	}
	pager := svc.(PagedService)
	page, err := pager.ListPage(t.Context(), sessionID, "", 60)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	require.Equal(t, ids[2], page.Messages[0].ID)
	require.True(t, page.HasMore)
	around, err := pager.ListPageAround(t.Context(), sessionID, ids[1], 3)
	require.NoError(t, err)
	require.Len(t, around.Messages, 1)
	require.Equal(t, ids[1], around.Messages[0].ID)
	text, err := pager.ListTextPage(t.Context(), sessionID, "", 60)
	require.NoError(t, err)
	require.Len(t, text.Messages, 3)
	for _, msg := range text.Messages {
		require.Len(t, msg.Parts, 1)
		require.Equal(t, "visible", msg.Content().Text)
	}
}

func TestMessagePageTimingCrossesPageBoundary(t *testing.T) {
	svc, sessionID := newTestService(t)
	user, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "start"}}})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant, Parts: []ContentPart{Finish{Reason: FinishReasonToolUse, Time: user.CreatedAt + 2}}})
	require.NoError(t, err)
	final, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant, Parts: []ContentPart{Finish{Reason: FinishReasonEndTurn, Time: user.CreatedAt + 17}}})
	require.NoError(t, err)
	page, err := svc.(PagedService).ListPage(t.Context(), sessionID, "", 1)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	require.True(t, page.HasMore)
	require.Equal(t, int64(17), page.Timing.Completed[final.ID])
	require.Equal(t, user.CreatedAt, page.Timing.StartedAt)
}

// A background job wake-up arrives as a hidden user message. It continues the
// request the user started, so the working timer must keep counting from that
// prompt instead of restarting at the wake-up.
func TestMessagePageTimingKeepsRunningAcrossWakeUp(t *testing.T) {
	svc, sessionID := newTestService(t)
	user, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "start"}}})
	require.NoError(t, err)
	first, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant, Parts: []ContentPart{Finish{Reason: FinishReasonEndTurn, Time: user.CreatedAt + 10}}})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), sessionID, CreateMessageParams{Role: User, Parts: []ContentPart{TextContent{Text: "Background job finished", Hidden: true}}})
	require.NoError(t, err)
	second, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant, Parts: []ContentPart{Finish{Reason: FinishReasonEndTurn, Time: user.CreatedAt + 25}}})
	require.NoError(t, err)
	page, err := svc.(PagedService).ListPage(t.Context(), sessionID, "", 60)
	require.NoError(t, err)
	require.Equal(t, int64(10), page.Timing.Completed[first.ID])
	require.Equal(t, int64(25), page.Timing.Completed[second.ID])
	require.Equal(t, user.CreatedAt, page.Timing.StartedAt)
}
