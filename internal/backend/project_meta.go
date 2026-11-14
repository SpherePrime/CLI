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
	dbPath   string
	lastUsed time.Time
	inUse    int
	retired  bool
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

	conn, release, err := acquireProjectMetaConn(absPath, dbPath)
	if err != nil {
		return nil, err
	}
	defer release()

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

func normalizeProjectMetaDBPath(dbPath string) string {
	if dbPath == "" {
		return ""
	}
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return filepath.Clean(dbPath)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

func acquireProjectMetaConn(path, dbPath string) (*sql.DB, func(), error) {
	cleanDBPath := normalizeProjectMetaDBPath(dbPath)
	now := time.Now()

	projectMeta.mu.Lock()
	if entry, ok := projectMeta.entries[path]; ok && !entry.retired {
		entry.inUse++
		entry.lastUsed = now
		projectMeta.mu.Unlock()
		return entry.conn, func() { releaseProjectMetaEntry(entry) }, nil
	}

	conn, err := db.OpenReadOnly(dbPath)
	if err != nil {
		projectMeta.mu.Unlock()
		return nil, nil, fmt.Errorf("failed to open %s: %w", dbPath, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	var evictedConn *sql.DB
	if len(projectMeta.entries) >= projectMetaMaxConns {
		var oldestUnusedPath string
		var oldestUnused time.Time
		var oldestAnyPath string
		var oldestAny time.Time
		for candidate, entry := range projectMeta.entries {
			if oldestAnyPath == "" || entry.lastUsed.Before(oldestAny) {
				oldestAnyPath = candidate
				oldestAny = entry.lastUsed
			}
			if entry.inUse == 0 && (oldestUnusedPath == "" || entry.lastUsed.Before(oldestUnused)) {
				oldestUnusedPath = candidate
				oldestUnused = entry.lastUsed
			}
		}
		evictPath := oldestUnusedPath
		if evictPath == "" {
			evictPath = oldestAnyPath
		}
		if old, ok := projectMeta.entries[evictPath]; ok {
			delete(projectMeta.entries, evictPath)
			old.retired = true
			if old.inUse == 0 {
				evictedConn = old.conn
			}
		}
	}

	entry := &projectMetaEntry{
		conn:     conn,
		dbPath:   cleanDBPath,
		lastUsed: now,
		inUse:    1,
	}
	projectMeta.entries[path] = entry
	projectMeta.mu.Unlock()

	if evictedConn != nil {
		_ = evictedConn.Close()
	}
	slog.Debug("Opened project metadata connection", "path", dbPath)
	return entry.conn, func() { releaseProjectMetaEntry(entry) }, nil
}

func releaseProjectMetaEntry(entry *projectMetaEntry) {
	projectMeta.mu.Lock()
	entry.inUse--
	entry.lastUsed = time.Now()
	shouldClose := entry.retired && entry.inUse == 0
	projectMeta.mu.Unlock()
	if shouldClose {
		_ = entry.conn.Close()
	}
}

func evictProjectMetaConn(projectPath, dataDir string) {
	absProject, err := filepath.Abs(projectPath)
	if err != nil {
		absProject = projectPath
	}
	resolvedProject, _ := resolveWorkspaceKey(projectPath)
	targetDBPath := normalizeProjectMetaDBPath(filepath.Join(dataDir, "prime.db"))
	closeProjectMetaConns(func(path string, entry *projectMetaEntry) bool {
		if path == absProject {
			return true
		}
		if resolvedProject != "" && path == resolvedProject {
			return true
		}
		if targetDBPath != "" && entry.dbPath == targetDBPath {
			return true
		}
		return false
	})
}

// CloseIdleProjectMetaConns drops cached read-only connections older than
// the TTL; the server calls it when it goes idle so no project database
// stays open in the background.
func CloseIdleProjectMetaConns() {
	closeProjectMetaConns(func(path string, entry *projectMetaEntry) bool {
		return entry.inUse == 0 && time.Since(entry.lastUsed) > projectMetaTTL
	})
}

// CloseAllProjectMetaConns closes every cached connection; tests call it
// so temp directories can be removed while the process still runs.
func CloseAllProjectMetaConns() {
	closeProjectMetaConns(func(string, *projectMetaEntry) bool { return true })
}

func closeProjectMetaConns(shouldClose func(path string, entry *projectMetaEntry) bool) {
	var toClose []*sql.DB
	projectMeta.mu.Lock()
	for path, entry := range projectMeta.entries {
		if shouldClose(path, entry) {
			delete(projectMeta.entries, path)
			entry.retired = true
			if entry.inUse == 0 {
				toClose = append(toClose, entry.conn)
			}
		}
	}
	projectMeta.mu.Unlock()
	for _, conn := range toClose {
		_ = conn.Close()
	}
}
