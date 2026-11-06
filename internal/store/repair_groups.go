package store

import (
	"context"
	"fmt"
	"regexp"
)

var internalGroupNamePattern = regexp.MustCompile(`(?i)^(group|grp)_[0-9a-f]{16,}$`)

func (s *Store) repairImportedGroupArtifacts(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(parent_id, '') FROM groups`)
	if err != nil {
		return fmt.Errorf("list groups for repair: %w", err)
	}
	type badGroup struct {
		id     string
		parent string
	}
	bad := make([]badGroup, 0, 4)
	for rows.Next() {
		var id, name, parent string
		if err := rows.Scan(&id, &name, &parent); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan group for repair: %w", err)
		}
		if internalGroupNamePattern.MatchString(name) {
			bad = append(bad, badGroup{id: id, parent: parent})
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate groups for repair: %w", err)
	}
	if len(bad) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin group repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range bad {
		if _, err := tx.ExecContext(ctx,
			`UPDATE hosts SET group_id = NULLIF(?, '') WHERE group_id = ?`, item.parent, item.id); err != nil {
			return fmt.Errorf("reparent repaired hosts: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET parent_id = NULLIF(?, '') WHERE parent_id = ?`, item.parent, item.id); err != nil {
			return fmt.Errorf("reparent repaired groups: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, item.id); err != nil {
			return fmt.Errorf("delete repaired group: %w", err)
		}
	}
	return tx.Commit()
}
