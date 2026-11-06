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

func (s *Store) ListTerminalRecordings(ctx context.Context) ([]model.TerminalRecording, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note FROM terminal_recordings ORDER BY julianday(started_at) DESC, started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query terminal recordings: %w", err)
	}
	defer rows.Close()
	return scanTerminalRecordingRows(rows)
}

func (s *Store) ListTerminalRecordingsPage(ctx context.Context, query string, offset, limit int) ([]model.TerminalRecording, int64, error) {
	query = strings.TrimSpace(query)
	where := ` WHERE mode <> 'full'`
	args := make([]any, 0, 3)
	if query != "" {

		where += ` AND LOWER(host_name || ' ' || address || ' ' || status || ' ' || note) LIKE ? ESCAPE '\'`
		pattern := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(query))
		args = append(args, "%"+pattern+"%")
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM terminal_recordings`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count terminal recordings: %w", err)
	}
	pageArgs := append(append([]any(nil), args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note FROM terminal_recordings`+where+` ORDER BY julianday(started_at) DESC, started_at DESC LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query terminal recording page: %w", err)
	}
	defer rows.Close()
	items, err := scanTerminalRecordingRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func scanTerminalRecordingRows(rows *sql.Rows) ([]model.TerminalRecording, error) {
	items := make([]model.TerminalRecording, 0)
	var err error
	for rows.Next() {
		var item model.TerminalRecording
		var startedAt, endedAt string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.HostID, &item.Mode, &item.Path, &startedAt, &endedAt, &item.Status, &item.Bytes, &item.EventCount, &item.HostName, &item.Address, &item.Note); err != nil {
			return nil, fmt.Errorf("scan terminal recording: %w", err)
		}
		item.StartedAt, err = parseTime(startedAt)
		if err != nil {
			return nil, fmt.Errorf("parse terminal recording start time: %w", err)
		}
		if endedAt != "" {
			item.EndedAt, err = parseTime(endedAt)
			if err != nil {
				return nil, fmt.Errorf("parse terminal recording end time: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate terminal recordings: %w", err)
	}
	return items, nil
}

func (s *Store) GetTerminalRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	var item model.TerminalRecording
	var startedAt, endedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note FROM terminal_recordings WHERE id = ?`, id).Scan(&item.ID, &item.SessionID, &item.HostID, &item.Mode, &item.Path, &startedAt, &endedAt, &item.Status, &item.Bytes, &item.EventCount, &item.HostName, &item.Address, &item.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TerminalRecording{}, ErrNotFound
	}
	if err != nil {
		return model.TerminalRecording{}, fmt.Errorf("get terminal recording: %w", err)
	}
	item.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return model.TerminalRecording{}, fmt.Errorf("parse terminal recording start time: %w", err)
	}
	if endedAt != "" {
		item.EndedAt, err = parseTime(endedAt)
		if err != nil {
			return model.TerminalRecording{}, fmt.Errorf("parse terminal recording end time: %w", err)
		}
	}
	return item, nil
}

func (s *Store) SaveTerminalRecording(ctx context.Context, item model.TerminalRecording) error {
	mode := strings.TrimSpace(item.Mode)
	if mode == "" {
		mode = "segment"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO terminal_recordings(id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET mode=excluded.mode,ended_at=excluded.ended_at,status=excluded.status,bytes=excluded.bytes,event_count=excluded.event_count,host_name=CASE WHEN excluded.host_name <> '' THEN excluded.host_name ELSE terminal_recordings.host_name END,address=CASE WHEN excluded.address <> '' THEN excluded.address ELSE terminal_recordings.address END,note=terminal_recordings.note`, item.ID, item.SessionID, item.HostID, mode, item.Path, formatTime(item.StartedAt), formatOptionalTime(item.EndedAt), item.Status, item.Bytes, item.EventCount, item.HostName, item.Address, item.Note)
	if err != nil {
		return fmt.Errorf("save terminal recording: %w", err)
	}
	return nil
}

func (s *Store) UpdateTerminalRecordingNote(ctx context.Context, id, note string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE terminal_recordings SET note = ? WHERE id = ?`, strings.TrimSpace(note), strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("update terminal recording note: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check terminal recording note update: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTerminalRecording(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM terminal_recordings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete terminal recording: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check terminal recording deletion: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetTerminalRecordings(ctx context.Context, ids []string) ([]model.TerminalRecording, error) {
	if len(ids) == 0 {
		return []model.TerminalRecording{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note FROM terminal_recordings WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("query terminal recordings by id: %w", err)
	}
	defer rows.Close()
	return scanTerminalRecordingRows(rows)
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}

func (s *Store) ListTerminalRecordingsBySessionIDs(ctx context.Context, sessionIDs []string) ([]model.TerminalRecording, error) {
	if len(sessionIDs) == 0 {
		return []model.TerminalRecording{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(sessionIDs)), ",")
	args := make([]any, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,host_id,mode,path,started_at,ended_at,status,bytes,event_count,host_name,address,note FROM terminal_recordings WHERE session_id IN (`+placeholders+`) ORDER BY julianday(started_at) DESC, started_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query terminal recordings by session: %w", err)
	}
	defer rows.Close()
	return scanTerminalRecordingRows(rows)
}
