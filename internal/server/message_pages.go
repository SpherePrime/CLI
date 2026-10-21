package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
)

func (c *controllerV1) handleMessagePage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Has("search") {
		matches, err := c.backend.SearchSessionMessages(r.Context(), r.PathValue("id"), r.PathValue("sid"), query.Get("search"))
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "session not found")
			return
		}
		if err != nil {
			c.handleError(w, r, err)
			return
		}
		jsonEncode(w, matches)
		return
	}
	limit := 60
	if query.Has("limit") {
		parsed, err := strconv.Atoi(query.Get("limit"))
		if err != nil || parsed < 1 || parsed > 200 {
			jsonError(w, http.StatusBadRequest, "limit must be between 1 and 200")
			return
		}
		limit = parsed
	}
	cursors := 0
	for _, key := range []string{"before", "after", "around"} {
		if query.Get(key) != "" {
			cursors++
		}
	}
	if cursors > 1 {
		jsonError(w, http.StatusBadRequest, "before, after and around cannot be used together")
		return
	}
	if query.Get("text_only") == "true" && (query.Get("after") != "" || query.Get("around") != "") {
		jsonError(w, http.StatusBadRequest, "text-only pages support before cursor")
		return
	}
	page, err := c.backend.ListSessionMessagePage(r.Context(), r.PathValue("id"), r.PathValue("sid"), query.Get("before"), query.Get("after"), query.Get("around"), limit, query.Get("text_only") == "true")
	if errors.Is(err, message.ErrInvalidCursor) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.MessagePage{
		Messages: messagesToProto(page.Messages), HasMore: page.HasMore,
		HasBefore: page.HasBefore, HasAfter: page.HasAfter, Before: page.Before, After: page.After,
		Timing: proto.MessagePageTiming{StartedAt: page.Timing.StartedAt, Completed: page.Timing.Completed},
	})
}
