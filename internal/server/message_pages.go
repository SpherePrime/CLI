package server

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
)

type messagePageParams struct {
	before   string
	after    string
	around   string
	limit    int
	textOnly bool
}

func parseMessagePageParams(query url.Values) (messagePageParams, string) {
	limit := 60
	if query.Has("limit") {
		parsed, err := strconv.Atoi(query.Get("limit"))
		if err != nil || parsed < 1 || parsed > 200 {
			return messagePageParams{}, "limit must be between 1 and 200"
		}
		limit = parsed
	}
	before := query.Get("before")
	after := query.Get("after")
	around := query.Get("around")
	cursors := 0
	for _, cursor := range []string{before, after, around} {
		if cursor != "" {
			cursors++
		}
	}
	if cursors > 1 {
		return messagePageParams{}, "before, after and around cannot be used together"
	}
	textOnly := query.Get("text_only") == "true"
	if textOnly && (after != "" || around != "") {
		return messagePageParams{}, "text-only pages support before cursor"
	}
	return messagePageParams{
		before:   before,
		after:    after,
		around:   around,
		limit:    limit,
		textOnly: textOnly,
	}, ""
}

func messagePageToProto(page message.Page) proto.MessagePage {
	return proto.MessagePage{
		Messages: messagesToProto(page.Messages), HasMore: page.HasMore,
		HasBefore: page.HasBefore, HasAfter: page.HasAfter, Before: page.Before, After: page.After,
		Timing: proto.MessagePageTiming{StartedAt: page.Timing.StartedAt, Completed: page.Timing.Completed},
	}
}

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
	params, errMsg := parseMessagePageParams(query)
	if errMsg != "" {
		jsonError(w, http.StatusBadRequest, errMsg)
		return
	}
	page, err := c.backend.ListSessionMessagePage(r.Context(), r.PathValue("id"), r.PathValue("sid"), params.before, params.after, params.around, params.limit, params.textOnly)
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
	jsonEncode(w, messagePageToProto(page))
}
