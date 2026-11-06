package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"changeme/internal/model"
)

const DefaultSessionHistoryLimit = 200

const (
	MinSessionHistoryLimit = 1
	MaxSessionHistoryLimit = 10_000
)

func (s *Store) ListSessionHistory(ctx context.Context) ([]model.SessionHistory, error) {

	rows, err := s.db.QueryContext(ctx, `SELECT id,host_id,host_name,address,started_at,ended_at,status,error_key,note,archived FROM session_history ORDER BY julianday(started_at) DESC, started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query session history: %w", err)
	}
	defer rows.Close()
	result := make([]model.SessionHistory, 0)
	for rows.Next() {
		var item model.SessionHistory
		var started, ended string
		var archived int
		if err := rows.Scan(&item.ID, &item.HostID, &item.HostName, &item.Address, &started, &ended, &item.Status, &item.ErrorKey, &item.Note, &archived); err != nil {
			return nil, fmt.Errorf("scan session history: %w", err)
		}
		item.Archived = archived != 0
		item.StartedAt, err = parseTime(started)
		if err != nil {
			return nil, fmt.Errorf("parse session history start: %w", err)
		}
		if ended != "" {
			item.EndedAt, err = parseTime(ended)
			if err != nil {
				return nil, fmt.Errorf("parse session history end: %w", err)
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session history: %w", err)
	}
	return result, nil
}

func (s *Store) SetSessionHistoryLimit(ctx context.Context, limit int) error {
	limit, err := normalizeSessionHistoryLimit(limit)
	if err != nil {
		return err
	}
	s.historyMu.Lock()
	s.historyLimit = limit
	s.historyMu.Unlock()
	return s.TrimSessionHistory(ctx, limit)
}

func (s *Store) sessionHistoryLimitValue() int {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()
	if s.historyLimit < MinSessionHistoryLimit || s.historyLimit > MaxSessionHistoryLimit {
		return DefaultSessionHistoryLimit
	}
	return s.historyLimit
}

func (s *Store) SaveSessionHistory(ctx context.Context, item model.SessionHistory) error {
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.HostID) == "" || item.StartedAt.IsZero() {
		return errors.New("session history identifiers are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save session history: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_history(id,host_id,host_name,address,started_at,ended_at,status,error_key,note,archived) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET host_name=excluded.host_name,address=excluded.address,ended_at=excluded.ended_at,status=excluded.status,error_key=excluded.error_key,note=excluded.note,archived=excluded.archived`, item.ID, item.HostID, item.HostName, item.Address, formatTime(item.StartedAt), formatOptionalTime(item.EndedAt), item.Status, item.ErrorKey, item.Note, boolInt(item.Archived)); err != nil {
		return fmt.Errorf("save session history: %w", err)
	}
	if err := trimSessionHistoryTx(ctx, tx, s.sessionHistoryLimitValue()); err != nil {
		return fmt.Errorf("trim session history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit session history: %w", err)
	}
	return nil
}

func (s *Store) CloseStaleSessionHistory(ctx context.Context, endedAt time.Time) (int64, error) {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE session_history SET status='interrupted', ended_at=? WHERE status='connected'`,
		formatTime(endedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("close stale session history: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("close stale session history: %w", err)
	}
	return affected, nil
}

func (s *Store) ArchiveSessionHistory(ctx context.Context, id string, archived bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin archive session history: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE session_history SET archived = ? WHERE id = ?`, boolInt(archived), id)
	if err != nil {
		return fmt.Errorf("archive session history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read archived session history count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	if err := trimSessionHistoryTx(ctx, tx, s.sessionHistoryLimitValue()); err != nil {
		return fmt.Errorf("trim session history after archive update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit archive session history: %w", err)
	}
	return nil
}

func (s *Store) TrimSessionHistory(ctx context.Context, limit int) error {
	limit, err := normalizeSessionHistoryLimit(limit)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trim session history: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := trimSessionHistoryTx(ctx, tx, limit); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trim session history: %w", err)
	}
	return nil
}

func normalizeSessionHistoryLimit(limit int) (int, error) {
	if limit < MinSessionHistoryLimit || limit > MaxSessionHistoryLimit {
		return 0, fmt.Errorf("%w: must be between %d and %d", ErrInvalidSessionHistoryLimit, MinSessionHistoryLimit, MaxSessionHistoryLimit)
	}
	return limit, nil
}

func trimSessionHistoryTx(ctx context.Context, tx *sql.Tx, limit int) error {
	limit, err := normalizeSessionHistoryLimit(limit)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM session_history WHERE archived = 0 AND id NOT IN (SELECT id FROM session_history WHERE archived = 0 ORDER BY julianday(started_at) DESC, started_at DESC, id DESC LIMIT ?)`, limit); err != nil {
		return fmt.Errorf("delete over-limit session history: %w", err)
	}
	return nil
}

func (s *Store) DeleteSessionHistory(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM session_history WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete session history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted session history count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetSessionHistory(ctx context.Context, id string) (model.SessionHistory, error) {
	var item model.SessionHistory
	var started, ended string
	var archived int
	err := s.db.QueryRowContext(ctx, `SELECT id,host_id,host_name,address,started_at,ended_at,status,error_key,note,archived FROM session_history WHERE id = ?`, strings.TrimSpace(id)).
		Scan(&item.ID, &item.HostID, &item.HostName, &item.Address, &started, &ended, &item.Status, &item.ErrorKey, &item.Note, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return model.SessionHistory{}, ErrNotFound
	}
	if err != nil {
		return model.SessionHistory{}, fmt.Errorf("get session history: %w", err)
	}
	item.StartedAt, err = parseTime(started)
	if err != nil {
		return model.SessionHistory{}, fmt.Errorf("parse session history start: %w", err)
	}
	if ended != "" {
		item.EndedAt, err = parseTime(ended)
		if err != nil {
			return model.SessionHistory{}, fmt.Errorf("parse session history end: %w", err)
		}
	}
	item.Archived = archived != 0
	return item, nil
}

func (s *Store) SetSessionHistoryArchived(ctx context.Context, ids []string, archived bool) (changed int, missing int, err error) {
	normalized := normalizeSessionHistoryIDs(ids)
	if len(normalized) == 0 {
		return 0, 0, nil
	}
	existing, err := s.sessionHistoryExistingIDs(ctx, normalized)
	if err != nil {
		return 0, 0, err
	}
	missing = len(normalized) - len(existing)
	if len(existing) == 0 {
		return 0, missing, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin batch archive session history: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	target := boolInt(archived)
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(existing)), ",")
	args := make([]any, 0, len(existing)+2)
	args = append(args, target, target)
	for _, id := range existing {
		args = append(args, id)
	}
	result, err := tx.ExecContext(ctx,
		`UPDATE session_history SET archived = ? WHERE archived <> ? AND id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("batch archive session history: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("read batch archived session history count: %w", err)
	}
	if err := trimSessionHistoryTx(ctx, tx, s.sessionHistoryLimitValue()); err != nil {
		return 0, 0, fmt.Errorf("trim session history after batch archive: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit batch archive session history: %w", err)
	}
	return int(affected), missing, nil
}

func (s *Store) sessionHistoryExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM session_history WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("list existing session history ids: %w", err)
	}
	defer rows.Close()
	existing := make([]string, 0, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan existing session history id: %w", err)
		}
		existing = append(existing, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing session history ids: %w", err)
	}
	return existing, nil
}

func normalizeSessionHistoryIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (s *Store) ClearUnarchivedSessionHistory(ctx context.Context) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin clear unarchived session history: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM session_history WHERE archived = 0`)
	if err != nil {
		return nil, fmt.Errorf("list unarchived session history: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan unarchived session history id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate unarchived session history: %w", err)
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close unarchived session history rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_history WHERE archived = 0`); err != nil {
		return nil, fmt.Errorf("clear unarchived session history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit clear unarchived session history: %w", err)
	}
	return ids, nil
}
