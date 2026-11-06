package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"changeme/internal/model"
)

func (s *Store) ListSnippetGroups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name FROM snippet_groups ORDER BY name COLLATE NOCASE,id")
	if err != nil {
		return nil, fmt.Errorf("list snippet groups: %w", err)
	}
	defer rows.Close()
	groups := make([]model.Group, 0)
	for rows.Next() {
		var group model.Group
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, fmt.Errorf("scan snippet group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Store) GetSnippetGroup(ctx context.Context, id string) (model.Group, error) {
	var group model.Group
	err := s.db.QueryRowContext(ctx, "SELECT id,name FROM snippet_groups WHERE id=?", id).Scan(&group.ID, &group.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return group, ErrNotFound
	}
	return group, err
}

func (s *Store) CreateSnippetGroup(ctx context.Context, group model.Group) (model.Group, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, fmt.Errorf("begin snippet group: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, "INSERT INTO snippet_groups(id,name,created_at,updated_at) VALUES(?,?,?,?) ON CONFLICT(name) DO NOTHING", group.ID, group.Name, formatTime(group.CreatedAt), formatTime(group.UpdatedAt))
	if err != nil {
		return model.Group{}, fmt.Errorf("insert snippet group: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return model.Group{}, fmt.Errorf("count snippet group: %w", err)
	}
	if count == 0 {
		return model.Group{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return model.Group{}, fmt.Errorf("commit snippet group: %w", err)
	}
	return group, nil
}

func (s *Store) RenameSnippetGroup(ctx context.Context, id, name string) (model.Group, error) {
	result, err := s.db.ExecContext(ctx, "UPDATE OR IGNORE snippet_groups SET name=?,updated_at=? WHERE id=?", name, formatTime(time.Now()), id)
	if err != nil {
		return model.Group{}, fmt.Errorf("rename snippet group: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return model.Group{}, fmt.Errorf("count renamed group: %w", err)
	}
	if count == 0 {
		if _, err := s.GetSnippetGroup(ctx, id); err != nil {
			return model.Group{}, err
		}
		return model.Group{}, ErrConflict
	}
	return s.GetSnippetGroup(ctx, id)
}

func (s *Store) DeleteSnippetGroup(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete snippet group: %w", err)
	}

	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "UPDATE snippets SET snippet_group_id=NULL WHERE snippet_group_id=?", id); err != nil {
		return fmt.Errorf("ungroup snippets: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM snippet_groups WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("delete snippet group: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted group: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
