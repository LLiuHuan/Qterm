package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"changeme/internal/model"
)

func (s *Store) ListGroups(ctx context.Context) ([]model.Group, error) {
	const query = `
		SELECT id, name, sort_order, COALESCE(parent_id, ''), created_at, updated_at
		FROM groups
		ORDER BY sort_order, name COLLATE NOCASE, id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query groups: %w", err)
	}
	defer rows.Close()

	groups := make([]model.Group, 0)
	for rows.Next() {
		var (
			group                model.Group
			createdAt, updatedAt string
		)
		if err := rows.Scan(&group.ID, &group.Name, &group.SortOrder, &group.ParentID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		group.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse group created time: %w", err)
		}
		group.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse group updated time: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

func reorderMixed(ctx context.Context, tx *sql.Tx, kind, id, parentID, targetID, position string) error {
	query := `SELECT kind, id FROM (
		SELECT 'group' AS kind, id, sort_order FROM groups WHERE parent_id IS ?
		UNION ALL
		SELECT 'host' AS kind, id, sort_order FROM hosts WHERE group_id IS ?
	) ORDER BY sort_order, kind, id`
	rows, err := tx.QueryContext(ctx, query, nullableID(parentID), nullableID(parentID))
	if err != nil {
		return err
	}
	type entry struct{ kind, id string }
	entries := make([]entry, 0)
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.kind, &item.id); err != nil {
			rows.Close()
			return err
		}
		if item.kind != kind || item.id != id {
			entries = append(entries, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	insertAt := len(entries)
	if targetID != "" && position != "inside" {
		for index, item := range entries {
			if item.id == targetID {
				insertAt = index
				if position == "after" {
					insertAt++
				}
				break
			}
		}
	}
	entries = append(entries, entry{})
	copy(entries[insertAt+1:], entries[insertAt:])
	entries[insertAt] = entry{kind: kind, id: id}
	for order, item := range entries {
		if item.kind == "group" {
			if _, err := tx.ExecContext(ctx, `UPDATE groups SET parent_id = NULLIF(?, ''), sort_order = ?, updated_at = ? WHERE id = ?`, parentID, order, formatTime(time.Now()), item.id); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE hosts SET group_id = NULLIF(?, ''), sort_order = ?, updated_at = ? WHERE id = ?`, parentID, order, formatTime(time.Now()), item.id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) ReorderHost(ctx context.Context, id, groupID, targetID, position string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder host: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if targetID != "" {
		var targetKind string
		if err := tx.QueryRowContext(ctx, `SELECT 'group' FROM groups WHERE id = ? UNION ALL SELECT 'host' FROM hosts WHERE id = ? LIMIT 1`, targetID, targetID).Scan(&targetKind); err == nil {
			if err := reorderMixed(ctx, tx, "host", id, groupID, targetID, position); err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	if groupID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, groupID).Scan(&count); err != nil {
			return fmt.Errorf("check host target group: %w", err)
		}
		if count == 0 {
			return ErrNotFound
		}
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE id = ?`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check host: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM hosts WHERE group_id IS ? AND id <> ? ORDER BY sort_order, id`, nullableID(groupID), id)
	if err != nil {
		return fmt.Errorf("list host siblings: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var sibling string
		if err := rows.Scan(&sibling); err != nil {
			rows.Close()
			return fmt.Errorf("scan host sibling: %w", err)
		}
		ids = append(ids, sibling)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate host siblings: %w", err)
	}
	rows.Close()
	insertAt := len(ids)
	if targetID != "" && position != "inside" {
		for i, sibling := range ids {
			if sibling == targetID {
				insertAt = i
				if position == "after" {
					insertAt++
				}
				break
			}
		}
	}
	ids = append(ids, "")
	copy(ids[insertAt+1:], ids[insertAt:])
	ids[insertAt] = id
	for order, hostID := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE hosts SET group_id = NULLIF(?, ''), sort_order = ?, updated_at = ? WHERE id = ?`, groupID, order, formatTime(time.Now()), hostID); err != nil {
			return fmt.Errorf("write host order: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit host reorder: %w", err)
	}
	return nil
}

func (s *Store) MoveHost(ctx context.Context, id, groupID string) error {
	return s.ReorderHost(ctx, id, groupID, "", "inside")
}

func (s *Store) ReorderGroup(ctx context.Context, id, parentID, targetID, position string) error {
	if id == "default" {
		return ErrProtectedGroup
	}
	if id == parentID {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder group: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check group: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	ancestor := parentID
	seen := map[string]struct{}{}
	for ancestor != "" {
		if ancestor == id {
			return ErrConflict
		}
		if _, ok := seen[ancestor]; ok {
			return ErrConflict
		}
		seen[ancestor] = struct{}{}
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(parent_id,'') FROM groups WHERE id = ?`, ancestor).Scan(&ancestor); err != nil {
			return ErrNotFound
		}
	}
	if targetID != "" {
		if err := reorderMixed(ctx, tx, "group", id, parentID, targetID, position); err != nil {
			return err
		}
		return tx.Commit()
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM groups WHERE parent_id IS ? AND id <> ? ORDER BY sort_order, id`, nullableID(parentID), id)
	if err != nil {
		return fmt.Errorf("list group siblings: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var sibling string
		if err := rows.Scan(&sibling); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, sibling)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	insertAt := len(ids)
	if targetID != "" && position != "inside" {
		for i, sibling := range ids {
			if sibling == targetID {
				insertAt = i
				if position == "after" {
					insertAt++
				}
				break
			}
		}
	}
	ids = append(ids, "")
	copy(ids[insertAt+1:], ids[insertAt:])
	ids[insertAt] = id
	for order, groupID := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET parent_id = NULLIF(?, ''), sort_order = ?, updated_at = ? WHERE id = ?`, parentID, order, formatTime(time.Now()), groupID); err != nil {
			return fmt.Errorf("write group order: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit group reorder: %w", err)
	}
	return nil
}

func (s *Store) MoveGroup(ctx context.Context, id, parentID string) error {
	if id == "default" {
		return ErrProtectedGroup
	}
	if id == parentID {
		return ErrConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin move group: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check moved group: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	if parentID != "" {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, parentID).Scan(&exists); err != nil {
			return fmt.Errorf("check group target parent: %w", err)
		}
		if exists == 0 {
			return ErrNotFound
		}
	}

	targetParentID := parentID
	ancestorID := parentID
	visited := make(map[string]struct{})
	for ancestorID != "" {
		if ancestorID == id {
			return ErrConflict
		}
		if _, duplicate := visited[ancestorID]; duplicate {
			return ErrConflict
		}
		visited[ancestorID] = struct{}{}

		var next string
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(parent_id,'') FROM groups WHERE id = ?`, ancestorID).Scan(&next)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read group parent chain: %w", err)
		}
		ancestorID = next
	}

	var nextGroupOrder int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM groups WHERE parent_id IS ? AND id <> ?`, nullableID(targetParentID), id).Scan(&nextGroupOrder); err != nil {
		return fmt.Errorf("next moved group sort order: %w", err)
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE groups SET parent_id = NULLIF(?, ''), sort_order = ?, updated_at = ? WHERE id = ?`,
		targetParentID,
		nextGroupOrder,
		formatTime(time.Now()),
		id,
	)
	if err != nil {
		return fmt.Errorf("move group: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read moved group count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit moved group: %w", err)
	}
	return nil
}

func (s *Store) CreateGroup(ctx context.Context, group model.Group) (model.Group, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, fmt.Errorf("begin create group: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE name = ? COLLATE NOCASE`, group.Name).Scan(&existingID)
	switch {
	case err == nil:
		return model.Group{}, ErrConflict
	case !errors.Is(err, sql.ErrNoRows):
		return model.Group{}, fmt.Errorf("check group name: %w", err)
	}
	if group.ParentID == group.ID {
		return model.Group{}, ErrConflict
	}
	if group.ParentID != "" {
		var parentExists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, group.ParentID).Scan(&parentExists); err != nil {
			return model.Group{}, fmt.Errorf("check group parent: %w", err)
		}
		if parentExists == 0 {
			return model.Group{}, ErrNotFound
		}
	}

	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM (SELECT sort_order FROM groups WHERE parent_id IS ? UNION ALL SELECT sort_order FROM hosts WHERE group_id IS ?)`, nullableID(group.ParentID), nullableID(group.ParentID)).Scan(&group.SortOrder); err != nil {
		return model.Group{}, fmt.Errorf("next group sort order: %w", err)
	}

	const insertGroup = `
		INSERT INTO groups (id, name, sort_order, parent_id, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?)`
	if _, err := tx.ExecContext(
		ctx,
		insertGroup,
		group.ID,
		group.Name,
		group.SortOrder,
		group.ParentID,
		formatTime(group.CreatedAt),
		formatTime(group.UpdatedAt),
	); err != nil {
		return model.Group{}, fmt.Errorf("insert group: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Group{}, fmt.Errorf("commit group: %w", err)
	}
	return group, nil
}

func (s *Store) UpdateGroup(ctx context.Context, group model.Group) (model.Group, error) {
	if group.ID == "default" {
		return model.Group{}, ErrProtectedGroup
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, fmt.Errorf("begin update group: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	var createdAt string
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM groups WHERE id = ?`, group.ID).Scan(&createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Group{}, ErrNotFound
		}
		return model.Group{}, fmt.Errorf("read group for update: %w", err)
	}

	var conflictID string
	err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM groups WHERE name = ? COLLATE NOCASE AND id <> ?`,
		group.Name,
		group.ID,
	).Scan(&conflictID)
	switch {
	case err == nil:
		return model.Group{}, ErrConflict
	case !errors.Is(err, sql.ErrNoRows):
		return model.Group{}, fmt.Errorf("check renamed group: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE groups SET name = ?, updated_at = ? WHERE id = ?`,
		group.Name,
		formatTime(group.UpdatedAt),
		group.ID,
	); err != nil {
		return model.Group{}, fmt.Errorf("update group: %w", err)
	}
	group.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return model.Group{}, fmt.Errorf("parse group creation time: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Group{}, fmt.Errorf("commit updated group: %w", err)
	}
	return group, nil
}

func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	if id == "default" {
		return ErrProtectedGroup
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete group: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	var childCount int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT (SELECT COUNT(*) FROM hosts WHERE group_id = ?) + (SELECT COUNT(*) FROM groups WHERE parent_id = ?)`,
		id,
		id,
	).Scan(&childCount); err != nil {
		return fmt.Errorf("count group children: %w", err)
	}
	if childCount > 0 {
		return ErrGroupNotEmpty
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted group count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit deleted group: %w", err)
	}
	return nil
}

func (s *Store) GetGroup(ctx context.Context, id string) (model.Group, error) {
	var g model.Group
	var c, u string
	err := s.db.QueryRowContext(ctx, `SELECT id,name,sort_order,COALESCE(parent_id,''),created_at,updated_at FROM groups WHERE id=?`, id).Scan(&g.ID, &g.Name, &g.SortOrder, &g.ParentID, &c, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return g, ErrNotFound
	}
	if err != nil {
		return g, err
	}
	g.CreatedAt, _ = parseTime(c)
	g.UpdatedAt, _ = parseTime(u)
	return g, nil
}

func ensureGroup(ctx context.Context, tx *sql.Tx, group model.Group) (string, string, error) {
	const insertGroup = `
		INSERT INTO groups (id, name, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO NOTHING`
	if _, err := tx.ExecContext(
		ctx,
		insertGroup,
		group.ID,
		group.Name,
		group.SortOrder,
		formatTime(group.CreatedAt),
		formatTime(group.UpdatedAt),
	); err != nil {
		return "", "", fmt.Errorf("ensure host group: %w", err)
	}

	var (
		id   string
		name string
	)
	if err := tx.QueryRowContext(ctx, `SELECT id, name FROM groups WHERE name = ? COLLATE NOCASE`, group.Name).Scan(&id, &name); err != nil {
		return "", "", fmt.Errorf("resolve host group: %w", err)
	}
	return id, name, nil
}
