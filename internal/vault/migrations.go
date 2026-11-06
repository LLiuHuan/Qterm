package vault

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

var (
	ErrSchemaTooNew = errors.New("vault schema is newer than this application")

	//go:embed migrations/*.sql
	migrationFiles embed.FS
)

func runMigrations(ctx context.Context, store *Store) error {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded vault migrations: %w", err)
	}

	var current int
	if err := store.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read vault schema version: %w", err)
	}
	if current > len(entries) {
		return fmt.Errorf("vault schema version %d exceeds supported version %d: %w", current, len(entries), ErrSchemaTooNew)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin vault migration transaction: %w", err)
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
			return fmt.Errorf("vault migration %q has version %d, want %d", entry.Name(), version, wantVersion)
		}

		migration, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read vault migration %q: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, string(migration)); err != nil {
			return fmt.Errorf("apply vault migration %q: %w", entry.Name(), err)
		}
		pragma := fmt.Sprintf("PRAGMA user_version = %d", version)
		if _, err := tx.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("record vault migration %q: %w", entry.Name(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vault migrations: %w", err)
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("vault migration %q has no numeric prefix", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("vault migration %q has invalid version", name)
	}
	return version, nil
}
