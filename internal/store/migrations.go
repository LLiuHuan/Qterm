package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

var (
	ErrSchemaTooNew = errors.New("database schema is newer than this application")

	//go:embed migrations/*.sql
	migrationFiles embed.FS
)

func runMigrations(ctx context.Context, store *Store) error {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var current int
	if err := store.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(entries) {
		return fmt.Errorf("schema version %d exceeds supported version %d: %w", current, len(entries), ErrSchemaTooNew)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	for i := current; i < len(entries); i++ {
		entry := entries[i]
		wantVersion := i + 1
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		if version != wantVersion {
			return fmt.Errorf("migration %q has version %d, want %d", entry.Name(), version, wantVersion)
		}

		migration, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		if version == 11 {
			if err := repairSnippetScope(ctx, tx); err != nil {
				return err
			}
		}
		if version == 24 {
			if err := repairHostPasswordSource(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, string(migration)); err != nil {
			return fmt.Errorf("apply migration %q: %w", entry.Name(), err)
		}
		if version == 40 {
			if err := store.repairProxyNamesAfterBackfill(ctx, tx); err != nil {
				return err
			}
		}
		pragma := fmt.Sprintf("PRAGMA user_version = %d", version)
		if _, err := tx.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("record migration %q: %w", entry.Name(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func repairHostPasswordSource(ctx context.Context, tx *sql.Tx) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('hosts') WHERE name='password_source'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect host password source: %w", err)
	}
	if exists != 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE hosts ADD COLUMN password_source TEXT NOT NULL DEFAULT 'none' CHECK (password_source IN ('none', 'prompt', 'saved'))`); err != nil {
		return fmt.Errorf("repair host password source: %w", err)
	}
	return nil
}

func repairSnippetScope(ctx context.Context, tx *sql.Tx) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('snippets') WHERE name='asset_group_id'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect snippet scope: %w", err)
	}
	if exists != 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE snippets ADD COLUMN asset_group_id TEXT REFERENCES groups(id) ON DELETE SET NULL`); err != nil {
		return fmt.Errorf("repair snippet scope: %w", err)
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q has no numeric prefix", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("migration %q has invalid version", name)
	}
	return version, nil
}
