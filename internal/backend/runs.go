package backend

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/runregistry"
	"log/slog"
	"sort"
	"time"
)

var ErrRunConflict = runregistry.ErrConflict
var ErrRunNotFound = runregistry.ErrNotFound

type registeredRun struct {
	record      proto.RunRecord
	fingerprint [32]byte
}

func (b *Backend) GetRun(workspaceID, runID string) (proto.RunRecord, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.RunRecord{}, err
	}
	if ws.runStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return ws.runStore.Get(ctx, runID)
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
	if ws.runStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return ws.runStore.List(ctx)
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
	if ws.runStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := ws.runStore.Finish(ctx, run.record)
		cancel()
		if err != nil {
			slog.Error("Failed to persist run completion", "run_id", completion.RunID, "error", err)
		}
	}
	ws.runs[completion.RunID] = run
}
