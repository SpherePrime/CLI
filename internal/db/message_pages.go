package db

import (
	"context"
	"database/sql"
	"strings"
)

type MessagePageParams struct {
	SessionID string
	Cursor    string
	After     bool
	Around    bool
	TextOnly  bool
	Limit     int
}

func (q *Queries) ListMessagePage(ctx context.Context, params MessagePageParams) ([]Message, error) {
	var cursorTime, cursorRow int64
	if params.Cursor != "" {
		err := q.db.QueryRowContext(ctx, `SELECT created_at, rowid FROM messages WHERE session_id = ? AND id = ?`, params.SessionID, params.Cursor).Scan(&cursorTime, &cursorRow)
		if err != nil {
			return nil, err
		}
	}
	query := `SELECT id, session_id, role, parts, model, created_at, updated_at, finished_at, provider, is_summary_message, prism_model_id, prism_model_name, prism_hypercredit_savings, prism_dollar_savings FROM messages WHERE session_id = ?`
	if params.TextOnly {
		query = strings.Replace(query, "role, parts, model", `role, COALESCE((SELECT json_group_array(json(value)) FROM json_each(parts) WHERE role IN ('user', 'assistant') AND json_extract(value, '$.type') = 'text' AND COALESCE(json_extract(value, '$.data.hidden'), 0) = 0), '[]'), model`, 1)
	}
	args := []any{params.SessionID}
	order := " DESC"
	if params.Around {
		query += ` AND rowid IN (SELECT rowid FROM (SELECT rowid FROM messages WHERE session_id = ? AND (created_at, rowid) < (?, ?) ORDER BY created_at DESC, rowid DESC LIMIT ?) UNION ALL SELECT rowid FROM (SELECT rowid FROM messages WHERE session_id = ? AND (created_at, rowid) >= (?, ?) ORDER BY created_at, rowid LIMIT ?))`
		args = append(args, params.SessionID, cursorTime, cursorRow, params.Limit/2, params.SessionID, cursorTime, cursorRow, params.Limit-params.Limit/2)
		order = " ASC"
	} else if params.Cursor != "" {
		comparison := " < "
		if params.After {
			comparison = " > "
			order = " ASC"
		}
		query += " AND (created_at, rowid)" + comparison + "(?, ?)"
		args = append(args, cursorTime, cursorRow)
	}
	query += " ORDER BY created_at" + order + ", rowid" + order + " LIMIT ?"
	args = append(args, params.Limit)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0, params.Limit)
	bytesRead := 0
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &item.Parts, &item.Model, &item.CreatedAt, &item.UpdatedAt, &item.FinishedAt, &item.Provider, &item.IsSummaryMessage, &item.PrismModelID, &item.PrismModelName, &item.PrismHypercreditSavings, &item.PrismDollarSavings); err != nil {
			return nil, err
		}
		if len(items) > 0 && bytesRead+len(item.Parts) > 8*1024*1024 {
			break
		}
		items = append(items, item)
		bytesRead += len(item.Parts)
	}
	return items, rows.Err()
}

type MessageSearch struct {
	Matches []string `json:"matches"`
	Total   int      `json:"total"`
}

func (q *Queries) SearchMessageText(ctx context.Context, sessionID, needle string) (MessageSearch, error) {
	result := MessageSearch{Matches: []string{}}
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return result, nil
	}
	rows, err := q.db.QueryContext(ctx, `SELECT m.id, (SELECT group_concat(COALESCE(json_extract(part.value, '$.data.text'), ''), char(10)) FROM json_each(m.parts) part WHERE json_extract(part.value, '$.type') = 'text' AND COALESCE(json_extract(part.value, '$.data.hidden'), 0) = 0) FROM messages m WHERE m.session_id = ? AND m.role IN ('user', 'assistant') ORDER BY m.created_at, m.rowid`, sessionID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var text sql.NullString
		if err := rows.Scan(&id, &text); err != nil {
			return result, err
		}
		if !strings.Contains(strings.ToLower(text.String), needle) {
			continue
		}
		result.Total++
		if len(result.Matches) < 10000 {
			result.Matches = append(result.Matches, id)
		}
	}
	return result, rows.Err()
}

func (q *Queries) MessagePageBounds(ctx context.Context, sessionID, first, last string) (bool, bool, error) {
	var before, after bool
	err := q.db.QueryRowContext(ctx, `SELECT
	EXISTS(SELECT 1 FROM messages WHERE session_id = ? AND (created_at, rowid) < (SELECT created_at, rowid FROM messages WHERE id = ? AND session_id = ?)),
	EXISTS(SELECT 1 FROM messages WHERE session_id = ? AND (created_at, rowid) > (SELECT created_at, rowid FROM messages WHERE id = ? AND session_id = ?))`, sessionID, first, sessionID, sessionID, last, sessionID).Scan(&before, &after)
	return before, after, err
}

type MessageTimingMetadata struct {
	ID           string
	Role         string
	CreatedAt    int64
	Shell        bool
	Hidden       bool
	FinishReason sql.NullString
	FinishTime   sql.NullInt64
}

func (q *Queries) ScanMessageTiming(ctx context.Context, sessionID, firstID, lastID string, visit func(MessageTimingMetadata)) error {
	// The boundary is the last finished run before the requested range. It is
	// only applied when a range is given: the whole-session pass (empty firstID)
	// has to walk back to the prompt that started the current request.
	rows, err := q.db.QueryContext(ctx, `WITH boundary AS MATERIALIZED (
	SELECT created_at, rowid AS message_row FROM messages
	WHERE session_id = ? AND role = 'assistant'
	AND (? != '' AND (created_at, rowid) < (SELECT created_at, rowid FROM messages WHERE session_id = ? AND id = ?))
	AND EXISTS (SELECT 1 FROM json_each(parts) WHERE json_extract(value, '$.type') = 'finish' AND json_extract(value, '$.data.reason') != 'tool_use')
	ORDER BY created_at DESC, rowid DESC LIMIT 1
	)
	SELECT id, role, created_at,
	CASE WHEN role = 'user' THEN EXISTS(SELECT 1 FROM json_each(parts) WHERE json_extract(value, '$.type') = 'shell_command') ELSE 0 END,
	CASE WHEN role = 'user' THEN EXISTS(SELECT 1 FROM json_each(parts) WHERE json_extract(value, '$.type') = 'text' AND COALESCE(json_extract(value, '$.data.hidden'), 0) = 1)
		AND NOT EXISTS(SELECT 1 FROM json_each(parts) WHERE json_extract(value, '$.type') = 'text' AND COALESCE(json_extract(value, '$.data.hidden'), 0) = 0) ELSE 0 END,
	CASE WHEN role = 'assistant' THEN (SELECT json_extract(value, '$.data.reason') FROM json_each(parts) WHERE json_extract(value, '$.type') = 'finish' LIMIT 1) END,
	CASE WHEN role = 'assistant' THEN (SELECT json_extract(value, '$.data.time') FROM json_each(parts) WHERE json_extract(value, '$.type') = 'finish' LIMIT 1) END
	FROM messages WHERE session_id = ? AND role IN ('user', 'assistant')
	AND (created_at, rowid) > (COALESCE((SELECT created_at FROM boundary), -1), COALESCE((SELECT message_row FROM boundary), -1))
	AND (? = '' OR (created_at, rowid) <= (SELECT created_at, rowid FROM messages WHERE session_id = ? AND id = ?))
	ORDER BY created_at, rowid`, sessionID, firstID, sessionID, firstID, sessionID, lastID, sessionID, lastID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item MessageTimingMetadata
		if err := rows.Scan(&item.ID, &item.Role, &item.CreatedAt, &item.Shell, &item.Hidden, &item.FinishReason, &item.FinishTime); err != nil {
			return err
		}
		visit(item)
	}
	return rows.Err()
}
