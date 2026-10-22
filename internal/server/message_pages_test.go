package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMessagePageAPICompatibilityAndSearch(t *testing.T) {
	c, workspaceID := buildBusyWorkspace(t, "unused", false)
	ws, err := c.backend.GetWorkspace(workspaceID)
	require.NoError(t, err)
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	queries := db.New(conn)
	ws.Sessions = session.NewService(queries, conn)
	ws.Messages = message.NewService(queries)
	sess, err := ws.Sessions.Create(t.Context(), "history")
	require.NoError(t, err)
	ids := make([]string, 9)
	for i := range ids {
		msg, err := ws.Messages.Create(t.Context(), sess.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "ПрИвЕт"}, message.TextContent{Text: "hidden text", Hidden: true}}})
		require.NoError(t, err)
		ids[i] = msg.ID
	}
	_, err = conn.ExecContext(t.Context(), "UPDATE messages SET created_at = 100 WHERE session_id = ?", sess.ID)
	require.NoError(t, err)
	request := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID+"/sessions/"+sess.ID+"/messages"+query, nil)
		req.SetPathValue("id", workspaceID)
		req.SetPathValue("sid", sess.ID)
		rec := httptest.NewRecorder()
		c.handleGetWorkspaceSessionMessages(rec, req)
		return rec
	}
	legacy := request("")
	require.Equal(t, http.StatusOK, legacy.Code)
	var all []proto.Message
	require.NoError(t, json.Unmarshal(legacy.Body.Bytes(), &all))
	require.Len(t, all, 9)
	var page proto.MessagePage
	latest := request("?limit=3")
	require.Equal(t, http.StatusOK, latest.Code)
	require.NoError(t, json.Unmarshal(latest.Body.Bytes(), &page))
	require.Equal(t, ids[6], page.Before)
	require.True(t, page.HasBefore)
	require.False(t, page.HasAfter)
	forward := request("?limit=3&after=" + ids[2])
	require.NoError(t, json.Unmarshal(forward.Body.Bytes(), &page))
	require.Equal(t, ids[3], page.Before)
	require.Equal(t, ids[5], page.After)
	require.True(t, page.HasBefore)
	require.True(t, page.HasAfter)
	center := request("?limit=3&around=" + ids[4])
	require.NoError(t, json.Unmarshal(center.Body.Bytes(), &page))
	require.Len(t, page.Messages, 3)
	require.Equal(t, ids[4], page.Messages[1].ID)
	for _, query := range []string{"?limit=0", "?limit=201", "?limit=abc", "?limit=3&before=missing", "?before=" + ids[0] + "&after=" + ids[1]} {
		require.Equal(t, http.StatusBadRequest, request(query).Code, query)
	}
	search := request("?search=%D0%BF%D1%80%D0%B8%D0%B2%D0%B5%D1%82")
	var matches db.MessageSearch
	require.NoError(t, json.Unmarshal(search.Body.Bytes(), &matches))
	require.Equal(t, ids, matches.Matches)
	require.Equal(t, 9, matches.Total)
	search = request("?search=hidden")
	require.NoError(t, json.Unmarshal(search.Body.Bytes(), &matches))
	require.Empty(t, matches.Matches)
	foreign, err := ws.Sessions.Create(t.Context(), "foreign")
	require.NoError(t, err)
	foreignMessage, err := ws.Messages.Create(t.Context(), foreign.ID, message.CreateMessageParams{Role: message.User})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, request("?limit=3&around="+foreignMessage.ID).Code)
}
