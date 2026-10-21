package backend

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/session"
)

// Project metadata support: the desktop needs the session list of every
// recent project right after startup, without opening full workspaces
// (no agent, no MCP processes, no config load, no data-dir lock). An open
// workspace stays authoritative; closed projects are read directly from
// their SQLite database through short-lived read-only connections.

const (
	projectMetaTTL      = 5 * time.Minute
	projectMetaMaxConns = 16
)

type projectMetaEntry struct {
	conn     *sql.DB
	lastUsed time.Time
}

type projectMetaCache struct {
	mu      sync.Mutex
	entries map[string]*projectMetaEntry
}

var projectMeta = &projectMetaCache{entries: map[string]*projectMetaEntry{}}

// ProjectSessions returns the top-level sessions of the project at the
// given absolute path. A missing database yields an empty list rather
// than an error.
func (b *Backend) ProjectSessions(ctx context.Context, projectPath string) ([]proto.Session, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path %q: %w", projectPath, err)
	}
	if !filepath.IsAbs(absPath) {
		return nil, fmt.Errorf("project path must be absolute: %s", projectPath)
	}

	// A resolved absolute path matches the workspace dedup key, so an open
	// project is served from its live session service.
	b.mu.Lock()
	id, ok := b.pathIndex[absPath]
	b.mu.Unlock()
	if ok {
		if ws, found := b.workspaces.Get(id); found {
			sessions, err := ws.Sessions.List(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]proto.Session, 0, len(sessions))
			for _, item := range sessions {
				out = append(out, sessionToProtoValue(item))
			}
			return out, nil
		}
	}

	dbPath := filepath.Join(absPath, ".prime", "prime.db")
	if _, err := os.Stat(dbPath); err != nil {
		return []proto.Session{}, nil
	}

	conn, err := acquireProjectMetaConn(absPath, dbPath)
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, projectSessionsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions from %s: %w", dbPath, err)
	}
	defer rows.Close()

	sessions := []proto.Session{}
	for rows.Next() {
		var item session.Session
		if err := scanProjectSession(rows, &item); err != nil {
			return nil, fmt.Errorf("failed to scan sessions from %s: %w", dbPath, err)
		}
		sessions = append(sessions, sessionToProtoValue(item))
	}
	return sessions, rows.Err()
}

// projectSessionsQuery mirrors db.ListSessions but skips the todos column:
// the read-only connection never runs migrations, so an older database that
// predates that column keeps working.
const projectSessionsQuery = `SELECT id, parent_session_id, title, message_count, prompt_tokens, completion_tokens, cost, updated_at, created_at, summary_message_id
FROM sessions
WHERE parent_session_id IS NULL
ORDER BY updated_at DESC`

func scanProjectSession(rows *sql.Rows, item *session.Session) error {
	var (
		id, title                   string
		parentID, summaryID         sql.NullString
		messageCount, prompt, done  int64
		cost                        float64
		updatedAt, createdAt        int64
	)
	if err := rows.Scan(
		&id,
		&parentID,
		&title,
		&messageCount,
		&prompt,
		&done,
		&cost,
		&updatedAt,
		&createdAt,
		&summaryID,
	); err != nil {
		return fmt.Errorf("failed to scan session row: %w", err)
	}
	*item = session.Session{
		ID:               id,
		ParentSessionID:  parentID.String,
		Title:            title,
		MessageCount:     messageCount,
		PromptTokens:     prompt,
		CompletionTokens: done,
		Cost:             cost,
		UpdatedAt:        updatedAt,
		CreatedAt:        createdAt,
		SummaryMessageID: summaryID.String,
	}
	return nil
}

func sessionToProtoValue(s session.Session) proto.Session {
	return proto.Session{
		ID:               s.ID,
		ParentSessionID:  s.ParentSessionID,
		Title:            s.Title,
		SummaryMessageID: s.SummaryMessageID,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

func acquireProjectMetaConn(path, dbPath string) (*sql.DB, error) {
	projectMeta.mu.Lock()
	defer projectMeta.mu.Unlock()

	if entry, ok := projectMeta.entries[path]; ok {
		entry.lastUsed = time.Now()
		return entry.conn, nil
	}

	conn, err := db.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", dbPath, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if len(projectMeta.entries) >= projectMetaMaxConns {
		var oldestPath string
		var oldest time.Time
		for candidate, entry := range projectMeta.entries {
			if oldestPath == "" || entry.lastUsed.Before(oldest) {
				oldestPath = candidate
				oldest = entry.lastUsed
			}
		}
		if old, ok := projectMeta.entries[oldestPath]; ok {
			_ = old.conn.Close()
			delete(projectMeta.entries, oldestPath)
		}
	}

	projectMeta.entries[path] = &projectMetaEntry{conn: conn, lastUsed: time.Now()}
	slog.Debug("Opened project metadata connection", "path", dbPath)
	return conn, nil
}

// CloseIdleProjectMetaConns drops cached read-only connections older than
// the TTL; the server calls it when it goes idle so no project database
// stays open in the background.
func CloseIdleProjectMetaConns() {
	closeProjectMetaConns(func(path string, entry *projectMetaEntry) bool {
		return time.Since(entry.lastUsed) > projectMetaTTL
	})
}

// CloseAllProjectMetaConns closes every cached connection; tests call it
// so temp directories can be removed while the process still runs.
func CloseAllProjectMetaConns() {
	closeProjectMetaConns(func(string, *projectMetaEntry) bool { return true })
}

func closeProjectMetaConns(shouldClose func(path string, entry *projectMetaEntry) bool) {
	projectMeta.mu.Lock()
	defer projectMeta.mu.Unlock()
	for path, entry := range projectMeta.entries {
		if shouldClose(path, entry) {
			_ = entry.conn.Close()
			delete(projectMeta.entries, path)
		}
	}
}
