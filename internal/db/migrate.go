package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const migrationLockID int64 = 0x7265676f

func runMigrations(ctx context.Context, connection *sql.DB) error {
	files, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found")
	}
	sort.Strings(files)

	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, tx)
	if err != nil {
		return err
	}

	for _, fileName := range files {
		version := migrationVersion(fileName)
		if _, exists := applied[version]; exists {
			continue
		}

		script, err := migrationFS.ReadFile(fileName)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", fileName, err)
		}

		statement := strings.TrimSpace(string(script))
		if statement != "" {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply migration %s: %w", fileName, err)
			}
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", fileName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}

func loadAppliedMigrations(ctx context.Context, tx *sql.Tx) (map[string]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func migrationVersion(fileName string) string {
	base := filepath.Base(fileName)
	if idx := strings.IndexByte(base, '_'); idx > 0 {
		return base[:idx]
	}

	return strings.TrimSuffix(base, filepath.Ext(base))
}
