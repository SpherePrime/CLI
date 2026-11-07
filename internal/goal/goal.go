package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusAchieved Status = "achieved"
)

var ErrEmptyText = errors.New("goal text is empty")
var ErrNoActiveGoal = errors.New("session has no active goal")

type Goal struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Status    Status `json:"status"`
	Summary   string `json:"summary,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (g Goal) Active() bool {
	return g.Status == StatusActive && strings.TrimSpace(g.Text) != ""
}

type Store interface {
	Get(ctx context.Context, sessionID string) (Goal, bool, error)
	Set(ctx context.Context, sessionID, text string) (Goal, error)
	Complete(ctx context.Context, sessionID, summary string) (Goal, error)
	Clear(ctx context.Context, sessionID string) error
}

type sqlStore struct {
	connection *sql.DB
	now        func() time.Time
}

func NewStore(connection *sql.DB) Store {
	return &sqlStore{connection: connection, now: time.Now}
}

func (s *sqlStore) Get(ctx context.Context, sessionID string) (Goal, bool, error) {
	row := s.connection.QueryRowContext(ctx, `SELECT session_id, text, status, summary, created_at, updated_at FROM session_goals WHERE session_id = ?`, sessionID)
	var stored Goal
	if err := row.Scan(&stored.SessionID, &stored.Text, &stored.Status, &stored.Summary, &stored.CreatedAt, &stored.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Goal{}, false, nil
		}
		return Goal{}, false, err
	}
	return stored, true, nil
}

func (s *sqlStore) Set(ctx context.Context, sessionID, text string) (Goal, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Goal{}, ErrEmptyText
	}
	timestamp := s.now().Unix()
	_, err := s.connection.ExecContext(ctx, `INSERT INTO session_goals(session_id, text, status, summary, created_at, updated_at) VALUES(?, ?, 'active', '', ?, ?)
ON CONFLICT(session_id) DO UPDATE SET text = excluded.text, status = 'active', summary = '', updated_at = excluded.updated_at`, sessionID, text, timestamp, timestamp)
	if err != nil {
		return Goal{}, err
	}
	stored, _, err := s.Get(ctx, sessionID)
	return stored, err
}

func (s *sqlStore) Complete(ctx context.Context, sessionID, summary string) (Goal, error) {
	result, err := s.connection.ExecContext(ctx, `UPDATE session_goals SET status = 'achieved', summary = ?, updated_at = ? WHERE session_id = ? AND status = 'active'`, strings.TrimSpace(summary), s.now().Unix(), sessionID)
	if err != nil {
		return Goal{}, err
	}
	if count, err := result.RowsAffected(); err != nil {
		return Goal{}, err
	} else if count == 0 {
		return Goal{}, ErrNoActiveGoal
	}
	stored, _, err := s.Get(ctx, sessionID)
	return stored, err
}

func (s *sqlStore) Clear(ctx context.Context, sessionID string) error {
	_, err := s.connection.ExecContext(ctx, `DELETE FROM session_goals WHERE session_id = ?`, sessionID)
	return err
}
