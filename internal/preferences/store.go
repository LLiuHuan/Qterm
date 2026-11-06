package preferences

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"changeme/internal/sqlitedsn"

	"modernc.org/sqlite"
)

const maxPreferenceValue = 16 << 20

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil || ctx.Err() != nil {
		if ctx == nil {
			return nil, errors.New("preference context is nil")
		}
		return nil, ctx.Err()
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) == filepath.VolumeName(path)+string(os.PathSeparator) {
		return nil, errors.New("preference database path is invalid")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create preference directory: %w", err)
	}

	connector, err := sqlite.NewConnector(sqlitedsn.Build(path, sqlitedsn.Options{ForeignKeys: true}))
	if err != nil {
		return nil, fmt.Errorf("create preference connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open preference database: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("restrict preference database: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize preference database: %w", err)
	}
	return store, nil
}

func (s *Store) Get(ctx context.Context, key string) (string, bool, error) {
	if err := validKey(key); err != nil {
		return "", false, err
	}
	if s == nil || s.db == nil {
		return "", false, errors.New("preference store is not ready")
	}
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read preference: %w", err)
	}
	return value, true, nil
}

func (s *Store) Set(ctx context.Context, key, value string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if len([]byte(value)) > maxPreferenceValue {
		return errors.New("preference value is too large")
	}
	if s == nil || s.db == nil {
		return errors.New("preference store is not ready")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES(?,?,CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=CURRENT_TIMESTAMP`, key, value); err != nil {
		return fmt.Errorf("write preference: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func validKey(key string) error {
	if key == "" || len(key) > 128 || key == "qterm." || !strings.HasPrefix(key, "qterm.") || strings.ContainsAny(key, "\x00/\\") {
		return errors.New("preference key is invalid")
	}
	return nil
}
