package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/history"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type spyHistoryService struct {
	*pubsub.Broker[history.File]
	versions []history.File
}

func (s *spyHistoryService) Create(ctx context.Context, sessionID, path, content string) (history.File, error) {
	f := history.File{SessionID: sessionID, Path: path, Content: content}
	s.versions = append(s.versions, f)
	return f, nil
}

func (s *spyHistoryService) CreateVersion(ctx context.Context, sessionID, path, content string) (history.File, error) {
	f := history.File{SessionID: sessionID, Path: path, Content: content, Version: int64(len(s.versions))}
	s.versions = append(s.versions, f)
	return f, nil
}

func (s *spyHistoryService) Get(ctx context.Context, id string) (history.File, error) {
	return history.File{}, nil
}

func (s *spyHistoryService) GetByPathAndSession(ctx context.Context, path, sessionID string) (history.File, error) {
	return history.File{}, nil
}

func (s *spyHistoryService) ListBySession(ctx context.Context, sessionID string) ([]history.File, error) {
	return append([]history.File(nil), s.versions...), nil
}

func (s *spyHistoryService) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, nil
}

func (s *spyHistoryService) Delete(ctx context.Context, id string) error {
	return nil
}

func (s *spyHistoryService) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	return nil
}

func TestWriteReplacedFileRecordsBeforeAndAfterHistory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.go")
	oldContent := "package main\nfunc hello() {}\n"
	newContent := "package main\nfunc hello() { println(\"hi\") }\n"
	require.NoError(t, os.WriteFile(filePath, []byte(oldContent), 0o644))

	spy := &spyHistoryService{}
	tracker := &mockEditFileTracker{}

	err := writeReplacedFile(t.Context(), spy, tracker, "sess-1", filePath, oldContent, newContent)
	require.NoError(t, err)

	diskBytes, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, newContent, string(diskBytes))

	require.Len(t, spy.versions, 2)
	require.Equal(t, oldContent, spy.versions[0].Content)
	require.Equal(t, newContent, spy.versions[1].Content)
	require.Equal(t, filePath, spy.versions[0].Path)
	require.Equal(t, filePath, spy.versions[1].Path)
	require.Equal(t, []string{filePath}, tracker.reads)
}

func TestRecordAffectedFileVersionsRecordsBeforeAndAfterRename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "rename.go")
	beforeContent := "package main\nfunc oldName() {}\n"
	afterContent := "package main\nfunc newName() {}\n"
	require.NoError(t, os.WriteFile(filePath, []byte(beforeContent), 0o644))

	spy := &spyHistoryService{}
	recordAffectedFileVersions(t.Context(), spy, "sess-2", []string{filePath}, "before rename")

	require.NoError(t, os.WriteFile(filePath, []byte(afterContent), 0o644))
	recordAffectedFileVersions(t.Context(), spy, "sess-2", []string{filePath}, "after rename")

	require.Len(t, spy.versions, 2)
	require.Equal(t, beforeContent, spy.versions[0].Content)
	require.Equal(t, afterContent, spy.versions[1].Content)
	require.Equal(t, "sess-2", spy.versions[1].SessionID)
}
