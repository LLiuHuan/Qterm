package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) ReorderSnippet(ctx context.Context, id, groupID, targetID, position string) error {
	if id == "" || (position != "before" && position != "after") || id == targetID {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin snippet reorder: %w", err)
	}

	defer func() { _ = tx.Rollback() }()
	var existing string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM snippets WHERE id=?", id).Scan(&existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if groupID != "" {
		if err := tx.QueryRowContext(ctx, "SELECT id FROM snippet_groups WHERE id=?", groupID).Scan(&existing); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM snippets WHERE COALESCE(snippet_group_id,'')=? AND id<>? ORDER BY sort_order,name COLLATE NOCASE,id", groupID, id)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var next string
		if err := rows.Scan(&next); err != nil {
			return errors.Join(err, rows.Close())
		}
		ids = append(ids, next)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	index := len(ids)
	if targetID != "" {
		index = -1
		for i, next := range ids {
			if next == targetID {
				index = i
				break
			}
		}
		if index < 0 {
			return ErrNotFound
		}
		if position == "after" {
			index++
		}
	}
	ids = append(ids, "")
	copy(ids[index+1:], ids[index:])
	ids[index] = id
	for i, next := range ids {
		result, err := tx.ExecContext(ctx, "UPDATE snippets SET snippet_group_id=?,sort_order=? WHERE id=?", nullableString(groupID), i, next)
		if err != nil {
			return fmt.Errorf("reorder snippet: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrNotFound
		}
	}
	return tx.Commit()
}
