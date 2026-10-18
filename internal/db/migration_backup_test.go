package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/pressly/goose/v3"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMigrationBackupIncludesCommittedWALAndPreservesOldSchema(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := openDB(filepath.Join(dir, "prime.db"))
	require.NoError(t, err)
	defer connection.Close()
	_, err = connection.ExecContext(ctx, `CREATE TABLE goose_db_version(id INTEGER PRIMARY KEY,version_id INTEGER NOT NULL,is_applied INTEGER NOT NULL); INSERT INTO goose_db_version VALUES(1,20260912000000,1); CREATE TABLE fixture(value TEXT); INSERT INTO fixture VALUES('saved WAL message')`)
	require.NoError(t, err)
	backup, err := prepareMigrationBackup(ctx, connection, dir)
	require.NoError(t, err)
	require.NotEmpty(t, backup)
	_, err = os.Stat(backup)
	require.NoError(t, err)
	_, err = connection.ExecContext(ctx, `UPDATE fixture SET value='changed after backup'`)
	require.NoError(t, err)
	copy, err := openDBReadOnly(backup)
	require.NoError(t, err)
	defer copy.Close()
	var value string
	require.NoError(t, copy.QueryRowContext(ctx, `SELECT value FROM fixture`).Scan(&value))
	require.Equal(t, "saved WAL message", value)
	var version int64
	require.NoError(t, copy.QueryRowContext(ctx, `SELECT version_id FROM goose_db_version`).Scan(&version))
	require.EqualValues(t, 20260912000000, version)
}

func TestNewerDatabaseSchemaIsRejectedWithoutBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := openDB(filepath.Join(dir, "prime.db"))
	require.NoError(t, err)
	defer connection.Close()
	_, err = connection.ExecContext(ctx, `CREATE TABLE goose_db_version(id INTEGER PRIMARY KEY,version_id INTEGER NOT NULL,is_applied INTEGER NOT NULL); INSERT INTO goose_db_version VALUES(1,20990101000000,1)`)
	require.NoError(t, err)
	_, err = prepareMigrationBackup(ctx, connection, dir)
	require.ErrorIs(t, err, ErrDatabaseNewer)
	files, err := filepath.Glob(filepath.Join(dir, "prime-before-migration-*.db"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestConnectBacksUpExistingSessionsBeforeUpgrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	previous, err := openDB(filepath.Join(dir, "prime.db"))
	require.NoError(t, err)
	require.NoError(t, initGoose())
	require.NoError(t, goose.UpTo(previous, "migrations", 20260912000000))
	_, err = New(previous).CreateSession(ctx, CreateSessionParams{ID: "preserved", Title: "existing history"})
	require.NoError(t, err)
	require.NoError(t, previous.Close())
	current, err := Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { Release(dir) })
	session, err := New(current).GetSessionByID(ctx, "preserved")
	require.NoError(t, err)
	require.Equal(t, "existing history", session.Title)
	backups, err := filepath.Glob(filepath.Join(dir, "prime-before-migration-*.db"))
	require.NoError(t, err)
	require.Len(t, backups, 1)
	copy, err := openDBReadOnly(backups[0])
	require.NoError(t, err)
	defer copy.Close()
	oldSession, err := New(copy).GetSessionByID(ctx, "preserved")
	require.NoError(t, err)
	require.Equal(t, session.Title, oldSession.Title)
	var hasRunTable bool
	require.NoError(t, copy.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='agent_runs')`).Scan(&hasRunTable))
	require.False(t, hasRunTable)
}

func TestConnectRejectsNewerDatabaseAndReleasesConnection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	previous, err := openDB(filepath.Join(dir, "prime.db"))
	require.NoError(t, err)
	_, err = previous.ExecContext(ctx, `CREATE TABLE goose_db_version(id INTEGER PRIMARY KEY,version_id INTEGER NOT NULL,is_applied INTEGER NOT NULL); INSERT INTO goose_db_version VALUES(1,20990101000000,1)`)
	require.NoError(t, err)
	require.NoError(t, previous.Close())
	connection, err := Connect(ctx, dir, WithDataDirLock(true))
	require.ErrorIs(t, err, ErrDatabaseNewer)
	require.Nil(t, connection)
	previous, err = openDB(filepath.Join(dir, "prime.db"))
	require.NoError(t, err)
	_, err = previous.ExecContext(ctx, `DROP TABLE goose_db_version`)
	require.NoError(t, err)
	require.NoError(t, previous.Close())
	connection, err = Connect(ctx, dir, WithDataDirLock(true))
	require.NoError(t, err)
	require.NotNil(t, connection)
	require.NoError(t, Release(dir))
}
