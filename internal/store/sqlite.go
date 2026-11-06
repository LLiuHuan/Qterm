package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"changeme/internal/sqlitedsn"

	"modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("record not found")

	ErrConflict = errors.New("record conflicts with existing data")

	ErrGroupNotEmpty = errors.New("group is not empty")

	ErrProtectedGroup = errors.New("group is protected")

	ErrInvalidSessionHistoryLimit = errors.New("invalid session history limit")
)

type Store struct {
	db    *sql.DB
	codec EncryptedPayloadCodec

	historyMu    sync.RWMutex
	historyLimit int
}

func Open(ctx context.Context, path string) (*Store, error) {
	return OpenWithCodec(ctx, path, nil)
}

func OpenWithCodec(ctx context.Context, path string, codec EncryptedPayloadCodec) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("restrict database directory permissions: %w", err)
		}
	}

	connector, err := sqlite.NewConnector(sqlitedsn.Build(path, sqlitedsn.Options{ForeignKeys: true}))
	if err != nil {
		return nil, fmt.Errorf("create SQLite connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, codec: codec, historyLimit: DefaultSessionHistoryLimit}
	opened := false
	defer func() {
		if !opened {
			_ = db.Close()
		}
	}()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect SQLite database: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, fmt.Errorf("restrict database permissions: %w", err)
		}
	}
	if err := runMigrations(ctx, store); err != nil {
		return nil, err
	}
	if codec != nil {
		if err := store.migrateEncryptedHostPayloads(ctx); err != nil {
			return nil, err
		}
	}

	if err := store.repairImportedGroupArtifacts(ctx); err != nil {
		return nil, err
	}
	if err := store.reconcileActiveRecordings(ctx); err != nil {
		return nil, err
	}
	if err := store.reconcileActiveSessionHistory(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}

	opened = true
	return store, nil
}

func (s *Store) reconcileActiveRecordings(ctx context.Context) error {
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE terminal_recordings SET status = ? WHERE status IN (?, ?)`,
		"interrupted", "recording", "paused",
	); err != nil {
		return fmt.Errorf("reconcile active terminal recordings: %w", err)
	}
	return nil
}

func (s *Store) reconcileActiveSessionHistory(ctx context.Context, detectedAt time.Time) error {
	if detectedAt.IsZero() {
		detectedAt = time.Now().UTC()
	}
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE session_history
		 SET status = ?,
		     ended_at = CASE WHEN ended_at = '' THEN ? ELSE ended_at END
		 WHERE status = ?`,
		"closed",
		formatTime(detectedAt),
		"connected",
	); err != nil {
		return fmt.Errorf("reconcile active session history: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
