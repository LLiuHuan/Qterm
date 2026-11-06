package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"changeme/internal/model"
)

const CommandHistoryLimit = 500

const CommandHistoryCommandLimit = 4096

func (s *Store) ListCommandHistory(ctx context.Context, hostID string) ([]model.CommandHistory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,host_id,host_name,session_id,command,executed_at,suppressed FROM command_history WHERE host_id = ? ORDER BY julianday(executed_at) DESC, id DESC LIMIT ?`, strings.TrimSpace(hostID), CommandHistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("query command history: %w", err)
	}
	defer rows.Close()
	items := make([]model.CommandHistory, 0)
	for rows.Next() {
		var item model.CommandHistory
		var executed string
		var suppressed int
		if err := rows.Scan(&item.ID, &item.HostID, &item.HostName, &item.SessionID, &item.Command, &executed, &suppressed); err != nil {
			return nil, fmt.Errorf("scan command history: %w", err)
		}
		item.ExecutedAt, err = parseTime(executed)
		if err != nil {
			return nil, fmt.Errorf("parse command history time: %w", err)
		}
		item.Suppressed = suppressed != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate command history: %w", err)
	}
	return items, nil
}

func (s *Store) SaveCommandHistory(ctx context.Context, item model.CommandHistory) error {
	command := strings.TrimSpace(item.Command)
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.HostID) == "" || command == "" || item.ExecutedAt.IsZero() {
		return errors.New("command history identifiers are required")
	}

	if utf8.RuneCountInString(command) > CommandHistoryCommandLimit {
		command = string([]rune(command)[:CommandHistoryCommandLimit])
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin command history transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO command_history(id,host_id,host_name,session_id,command,executed_at,suppressed) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET command=excluded.command,executed_at=excluded.executed_at,suppressed=excluded.suppressed`, item.ID, item.HostID, item.HostName, item.SessionID, command, formatTime(item.ExecutedAt), boolInt(item.Suppressed)); err != nil {
		return fmt.Errorf("save command history: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM command_history WHERE host_id = ? AND id <> ? AND id NOT IN (SELECT id FROM command_history WHERE host_id = ? ORDER BY julianday(executed_at) DESC, id DESC LIMIT ?)`, item.HostID, item.ID, item.HostID, CommandHistoryLimit); err != nil {
		return fmt.Errorf("trim command history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit command history: %w", err)
	}
	return nil
}

func (s *Store) SuppressCommandHistory(ctx context.Context, hostID, command string) (int64, error) {
	hostID = strings.TrimSpace(hostID)
	command = strings.TrimSpace(command)
	if hostID == "" || command == "" {
		return 0, errors.New("command history host and command are required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE command_history SET suppressed = 1 WHERE host_id = ? AND command = ?`, hostID, command)
	if err != nil {
		return 0, fmt.Errorf("suppress command history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read suppressed command history count: %w", err)
	}
	return count, nil
}

func (s *Store) DeleteCommandHistory(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM command_history WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete command history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted command history count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ClearCommandHistory(ctx context.Context, hostID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM command_history WHERE host_id = ?`, strings.TrimSpace(hostID))
	if err != nil {
		return 0, fmt.Errorf("clear command history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read cleared command history count: %w", err)
	}
	return count, nil
}
