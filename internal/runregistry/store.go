package runregistry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/SpherePrime/CLI/internal/proto"
)

var ErrConflict = errors.New("run_id already belongs to a different request")
var ErrNotFound = errors.New("run not found")

type Store struct {
	connection    *sql.DB
	workspacePath string
	processID     string
	workspaceID   string
}

func New(ctx context.Context, connection *sql.DB, workspacePath, processID, workspaceID string) (*Store, error) {
	store := &Store{connection: connection, workspacePath: workspacePath, processID: processID, workspaceID: workspaceID}
	_, err := connection.ExecContext(ctx, `UPDATE agent_runs SET status='interrupted', finished_at=?, error='server restarted before run completion' WHERE workspace_path=? AND status='running' AND process_id<>?`, time.Now().UTC().Format(time.RFC3339Nano), workspacePath, processID)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Accept(ctx context.Context, message proto.AgentMessage) (bool, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return false, err
	}
	fingerprint := sha256.Sum256(data)
	result, err := s.connection.ExecContext(ctx, `INSERT INTO agent_runs(workspace_path,run_id,session_id,request_fingerprint,process_id,workspace_instance_id,status,accepted_at) VALUES(?,?,?,?,?,?,'running',?) ON CONFLICT(workspace_path,run_id) DO NOTHING`, s.workspacePath, message.RunID, message.SessionID, fingerprint[:], s.processID, s.workspaceID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	var previous []byte
	if err := s.connection.QueryRowContext(ctx, `SELECT request_fingerprint FROM agent_runs WHERE workspace_path=? AND run_id=?`, s.workspacePath, message.RunID).Scan(&previous); err != nil {
		return false, err
	}
	if !bytes.Equal(previous, fingerprint[:]) {
		return false, ErrConflict
	}
	return false, nil
}

const recordColumns = `run_id,session_id,status,accepted_at,finished_at,error,message_id,response_text`

type rowScanner interface{ Scan(...any) error }

func scanRecord(row rowScanner) (proto.RunRecord, error) {
	var record proto.RunRecord
	var acceptedAt string
	var finishedAt sql.NullString
	err := row.Scan(&record.RunID, &record.SessionID, &record.Status, &acceptedAt, &finishedAt, &record.Error, &record.MessageID, &record.Text)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	record.AcceptedAt, err = time.Parse(time.RFC3339Nano, acceptedAt)
	if err != nil {
		return record, err
	}
	if finishedAt.Valid {
		value, err := time.Parse(time.RFC3339Nano, finishedAt.String)
		if err != nil {
			return record, err
		}
		record.FinishedAt = &value
	}
	return record, nil
}

func (s *Store) Get(ctx context.Context, runID string) (proto.RunRecord, error) {
	return scanRecord(s.connection.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM agent_runs WHERE workspace_path=? AND run_id=?`, s.workspacePath, runID))
}

func (s *Store) List(ctx context.Context) ([]proto.RunRecord, error) {
	rows, err := s.connection.QueryContext(ctx, `SELECT `+recordColumns+` FROM agent_runs WHERE workspace_path=? ORDER BY accepted_at,run_id`, s.workspacePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []proto.RunRecord{}
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) Finish(ctx context.Context, record proto.RunRecord) error {
	_, err := s.connection.ExecContext(ctx, `UPDATE agent_runs SET status=?,finished_at=?,error=?,message_id=?,response_text=? WHERE workspace_path=? AND run_id=? AND status='running'`, record.Status, time.Now().UTC().Format(time.RFC3339Nano), record.Error, record.MessageID, record.Text, s.workspacePath, record.RunID)
	return err
}

func (s *Store) Interrupt(ctx context.Context) error {
	_, err := s.connection.ExecContext(ctx, `UPDATE agent_runs SET status='interrupted',finished_at=?,error='workspace closed before run completion' WHERE workspace_path=? AND workspace_instance_id=? AND process_id=? AND status='running'`, time.Now().UTC().Format(time.RFC3339Nano), s.workspacePath, s.workspaceID, s.processID)
	return err
}
