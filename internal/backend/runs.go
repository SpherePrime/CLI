package backend

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/proto"
	"sort"
	"time"
)

var ErrRunConflict = errors.New("run_id already belongs to a different request")
var ErrRunNotFound = errors.New("run not found")

type registeredRun struct {
	record      proto.RunRecord
	fingerprint [32]byte
}

func (b *Backend) GetRun(workspaceID, runID string) (proto.RunRecord, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.RunRecord{}, err
	}
	ws.runMu.Lock()
	defer ws.runMu.Unlock()
	run, ok := ws.runs[runID]
	if !ok {
		return proto.RunRecord{}, ErrRunNotFound
	}
	return run.record, nil
}

func (b *Backend) ListRuns(workspaceID string) ([]proto.RunRecord, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	ws.runMu.Lock()
	defer ws.runMu.Unlock()
	runs := make([]proto.RunRecord, 0, len(ws.runs))
	for _, run := range ws.runs {
		runs = append(runs, run.record)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].AcceptedAt.Before(runs[j].AcceptedAt) })
	return runs, nil
}

func requestFingerprint(msg proto.AgentMessage) [32]byte {
	data, _ := json.Marshal(msg)
	return sha256.Sum256(data)
}

func (ws *Workspace) finishRun(completion notify.RunComplete) {
	ws.runMu.Lock()
	defer ws.runMu.Unlock()
	run, ok := ws.runs[completion.RunID]
	if !ok || run.record.FinishedAt != nil {
		return
	}
	now := time.Now().UTC()
	run.record.FinishedAt = &now
	run.record.Status = "completed"
	if completion.Error != "" {
		run.record.Status = "failed"
	}
	if completion.Cancelled {
		run.record.Status = "cancelled"
	}
	run.record.Error = completion.Error
	run.record.MessageID = completion.MessageID
	run.record.Text = completion.Text
	ws.runs[completion.RunID] = run
}
