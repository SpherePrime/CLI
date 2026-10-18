package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/SpherePrime/CLI/vendordeps/pressly/goose/v3"
)

var ErrDatabaseNewer = errors.New("database schema is newer than this Prime version")

func prepareMigrationBackup(ctx context.Context, connection *sql.DB, dataDir string) (string, error) {
	migrations, err := goose.CollectMigrations("migrations", 0, math.MaxInt64)
	if err != nil {
		return "", err
	}
	if len(migrations) == 0 {
		return "", nil
	}
	latest := migrations[len(migrations)-1].Version
	var hasVersionTable bool
	if err := connection.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='goose_db_version')`).Scan(&hasVersionTable); err != nil {
		return "", err
	}
	var current int64
	if hasVersionTable {
		if err := connection.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id),0) FROM goose_db_version g WHERE is_applied=1 AND id=(SELECT MAX(id) FROM goose_db_version WHERE version_id=g.version_id)`).Scan(&current); err != nil {
			return "", err
		}
	}
	if current > latest {
		return "", fmt.Errorf("%w: database %d, supported %d", ErrDatabaseNewer, current, latest)
	}
	if current == latest {
		return "", nil
	}
	var tableCount int
	if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		return "", err
	}
	if tableCount == 0 {
		return "", nil
	}
	file, err := os.CreateTemp(dataDir, fmt.Sprintf("prime-before-migration-%d-to-%d-*.db", current, latest))
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			os.Remove(path)
		}
	}()
	if _, err := connection.ExecContext(ctx, `VACUUM main INTO ?`, path); err != nil {
		return "", fmt.Errorf("create consistent database backup: %w", err)
	}
	copy, err := openDBReadOnly(path)
	if err != nil {
		return "", err
	}
	var check string
	err = copy.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check)
	copy.Close()
	if err != nil {
		return "", err
	}
	if check != "ok" {
		return "", fmt.Errorf("database backup validation failed: %s", check)
	}
	synced, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	err = synced.Sync()
	synced.Close()
	if err != nil {
		return "", err
	}
	complete = true
	return path, nil
}
