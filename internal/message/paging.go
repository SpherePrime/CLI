package message

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/SpherePrime/CLI/internal/db"
)

var ErrInvalidCursor = errors.New("message cursor does not belong to the session")

type PageTiming struct {
	StartedAt int64            `json:"started_at,omitempty"`
	Completed map[string]int64 `json:"completed"`
}

type Page struct {
	Messages  []Message
	HasMore   bool
	HasBefore bool
	HasAfter  bool
	Before    string
	After     string
	Timing    PageTiming
}

type PagedService interface {
	ListPage(context.Context, string, string, int) (Page, error)
	ListPageAfter(context.Context, string, string, int) (Page, error)
	ListPageAround(context.Context, string, string, int) (Page, error)
	SearchText(context.Context, string, string) (db.MessageSearch, error)
	ListTextPage(context.Context, string, string, int) (Page, error)
}

type pageQuerier interface {
	ListMessagePage(context.Context, db.MessagePageParams) ([]db.Message, error)
	MessagePageBounds(context.Context, string, string, string) (bool, bool, error)
	ScanMessageTiming(context.Context, string, string, string, func(db.MessageTimingMetadata)) error
	SearchMessageText(context.Context, string, string) (db.MessageSearch, error)
}

func (s *service) ListPage(ctx context.Context, sessionID, before string, limit int) (Page, error) {
	return s.listPage(ctx, sessionID, before, limit, false, false, false)
}

func (s *service) ListPageAfter(ctx context.Context, sessionID, after string, limit int) (Page, error) {
	return s.listPage(ctx, sessionID, after, limit, true, false, false)
}

func (s *service) ListPageAround(ctx context.Context, sessionID, around string, limit int) (Page, error) {
	return s.listPage(ctx, sessionID, around, limit, false, true, false)
}

func (s *service) ListTextPage(ctx context.Context, sessionID, before string, limit int) (Page, error) {
	return s.listPage(ctx, sessionID, before, limit, false, false, true)
}

func (s *service) SearchText(ctx context.Context, sessionID, needle string) (db.MessageSearch, error) {
	queries, ok := s.q.(pageQuerier)
	if !ok {
		return db.MessageSearch{}, fmt.Errorf("message search is unavailable")
	}
	if err := s.FlushAll(ctx); err != nil {
		return db.MessageSearch{}, err
	}
	return queries.SearchMessageText(ctx, sessionID, needle)
}

func (s *service) listPage(ctx context.Context, sessionID, cursor string, limit int, after, around, textOnly bool) (Page, error) {
	page := Page{Messages: []Message{}, Timing: PageTiming{Completed: map[string]int64{}}}
	if limit < 1 || limit > 200 {
		return page, fmt.Errorf("message page limit must be between 1 and 200")
	}
	queries, ok := s.q.(pageQuerier)
	if !ok {
		return page, fmt.Errorf("message pagination is unavailable")
	}
	if err := s.FlushAll(ctx); err != nil {
		return page, err
	}
	queryLimit := limit + 1
	if around {
		queryLimit = limit
	}
	rows, err := queries.ListMessagePage(ctx, db.MessagePageParams{SessionID: sessionID, Cursor: cursor, After: after, Around: around, TextOnly: textOnly, Limit: queryLimit})
	if errors.Is(err, sql.ErrNoRows) {
		return page, ErrInvalidCursor
	}
	if err != nil {
		return page, err
	}
	if around && !slices.ContainsFunc(rows, func(row db.Message) bool { return row.ID == cursor }) {
		rows, err = queries.ListMessagePage(ctx, db.MessagePageParams{SessionID: sessionID, Cursor: cursor, Around: true, Limit: 1})
		if err != nil {
			return page, err
		}
	}
	page.HasMore = len(rows) > limit
	if page.HasMore {
		rows = rows[:limit]
	}
	if !after && !around {
		slices.Reverse(rows)
	}
	wanted := make(map[string]bool, len(rows))
	for _, row := range rows {
		msg, err := s.fromDBItem(row)
		if err != nil {
			return page, err
		}
		page.Messages = append(page.Messages, msg)
		if msg.Role == Assistant {
			wanted[msg.ID] = true
		}
	}
	if len(rows) > 0 {
		page.Before, page.After = rows[0].ID, rows[len(rows)-1].ID
		page.HasBefore, page.HasAfter, err = queries.MessagePageBounds(ctx, sessionID, page.Before, page.After)
		if err != nil {
			return page, err
		}
		if after {
			page.HasMore = page.HasAfter
		} else {
			page.HasMore = page.HasBefore
		}
		if around {
			page.HasMore = page.HasBefore || page.HasAfter
		}
	}
	visit := func(item db.MessageTimingMetadata) {
		if item.Shell {
			return
		}
		if page.Timing.StartedAt == 0 {
			page.Timing.StartedAt = item.CreatedAt
		}
		if item.Role != "assistant" || !item.FinishReason.Valid || item.FinishReason.String == string(FinishReasonToolUse) {
			return
		}
		if wanted[item.ID] && item.FinishTime.Valid && item.FinishTime.Int64 > 0 && page.Timing.StartedAt > 0 {
			page.Timing.Completed[item.ID] = max(int64(0), item.FinishTime.Int64-page.Timing.StartedAt)
		}
		page.Timing.StartedAt = 0
	}
	if textOnly {
		return page, nil
	}
	if len(wanted) > 0 {
		err = queries.ScanMessageTiming(ctx, sessionID, page.Before, page.After, visit)
		if err != nil {
			return page, err
		}
	}
	page.Timing.StartedAt = 0
	err = queries.ScanMessageTiming(ctx, sessionID, "", "", visit)
	return page, err
}
