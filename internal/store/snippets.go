package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"changeme/internal/model"
)

func (s *Store) ListSnippets(ctx context.Context) ([]model.Snippet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT s.id,s.name,s.content,COALESCE(s.snippet_group_id,''),COALESCE(sg.name,''),COALESCE(s.asset_group_id,''),COALESCE(ag.name,''),s.auto_execute,s.host_ids,s.created_at,s.updated_at,s.icon,s.color,s.pinned FROM snippets s LEFT JOIN snippet_groups sg ON sg.id=s.snippet_group_id LEFT JOIN groups ag ON ag.id=s.asset_group_id ORDER BY COALESCE(sg.name,''),s.sort_order,s.name COLLATE NOCASE,s.id`)
	if err != nil {
		return nil, fmt.Errorf("query snippets: %w", err)
	}
	defer rows.Close()
	result := make([]model.Snippet, 0)
	for rows.Next() {
		var v model.Snippet
		var auto int
		var created, updated, hostIDs string
		if err := rows.Scan(&v.ID, &v.Name, &v.Content, &v.GroupID, &v.Group, &v.AssetGroupID, &v.AssetGroup, &auto, &hostIDs, &created, &updated, &v.Icon, &v.Color, &v.Pinned); err != nil {
			return nil, fmt.Errorf("scan snippet: %w", err)
		}
		v.AutoExecute = auto != 0
		_ = json.Unmarshal([]byte(hostIDs), &v.HostIDs)
		v.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		v.UpdatedAt, err = parseTime(updated)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *Store) CreateSnippet(ctx context.Context, v model.Snippet) (model.Snippet, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO snippets(id,name,content,snippet_group_id,asset_group_id,auto_execute,host_ids,created_at,updated_at,icon,color,pinned,sort_order) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,(SELECT COALESCE(MAX(sort_order),-1)+1 FROM snippets))`, v.ID, v.Name, v.Content, nullableString(v.GroupID), nullableString(v.AssetGroupID), boolInt(v.AutoExecute), encodeHostIDs(v.HostIDs), formatTime(v.CreatedAt), formatTime(v.UpdatedAt), v.Icon, v.Color, boolInt(v.Pinned))
	if err != nil {
		return model.Snippet{}, fmt.Errorf("create snippet: %w", err)
	}
	return v, nil
}

func (s *Store) UpdateSnippet(ctx context.Context, v model.Snippet) (model.Snippet, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE snippets SET name=?,content=?,snippet_group_id=?,asset_group_id=?,auto_execute=?,host_ids=?,updated_at=?,icon=?,color=?,pinned=? WHERE id=?`, v.Name, v.Content, nullableString(v.GroupID), nullableString(v.AssetGroupID), boolInt(v.AutoExecute), encodeHostIDs(v.HostIDs), formatTime(v.UpdatedAt), v.Icon, v.Color, boolInt(v.Pinned), v.ID)
	if err != nil {
		return model.Snippet{}, fmt.Errorf("update snippet: %w", err)
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return model.Snippet{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) DeleteSnippet(ctx context.Context, id string) error {
	r, err := s.db.ExecContext(ctx, `DELETE FROM snippets WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetSnippet(ctx context.Context, id string) (model.Snippet, error) {
	var v model.Snippet
	var auto int
	var c, u, hostIDs string
	err := s.db.QueryRowContext(ctx, `SELECT id,name,content,COALESCE(snippet_group_id,''),auto_execute,host_ids,COALESCE(asset_group_id,''),created_at,updated_at,icon,color,pinned FROM snippets WHERE id=?`, id).Scan(&v.ID, &v.Name, &v.Content, &v.GroupID, &auto, &hostIDs, &v.AssetGroupID, &c, &u, &v.Icon, &v.Color, &v.Pinned)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	v.AutoExecute = auto != 0
	_ = json.Unmarshal([]byte(hostIDs), &v.HostIDs)
	v.CreatedAt, _ = parseTime(c)
	v.UpdatedAt, _ = parseTime(u)
	return v, nil
}
