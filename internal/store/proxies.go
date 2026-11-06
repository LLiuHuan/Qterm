package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"changeme/internal/model"
)

const proxyColumns = `id, name, mode, host, port, username, created_at, updated_at`

const proxyReadColumns = proxyColumns + `, encrypted_payload`

func (s *Store) scanProxy(row interface{ Scan(...any) error }) (model.ProxyRecord, error) {
	var proxy model.ProxyRecord
	var payload []byte
	if err := row.Scan(
		&proxy.ID,
		&proxy.Name,
		&proxy.Mode,
		&proxy.Host,
		&proxy.Port,
		&proxy.Username,
		&proxy.CreatedAt,
		&proxy.UpdatedAt,
		&payload,
	); err != nil {
		return model.ProxyRecord{}, err
	}
	proxy.PasswordConfigured = len(payload) != 0
	clear(payload)
	return proxy, nil
}

func (s *Store) ListProxies(ctx context.Context) ([]model.ProxyRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+proxyReadColumns+` FROM proxies ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("query proxies: %w", err)
	}
	defer rows.Close()

	proxies := make([]model.ProxyRecord, 0)
	for rows.Next() {
		proxy, err := s.scanProxy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proxy: %w", err)
		}
		proxies = append(proxies, proxy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, nil
}

func (s *Store) ProxyByID(ctx context.Context, id string) (model.ProxyRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.ProxyRecord{}, ErrNotFound
	}
	proxy, err := s.scanProxy(s.db.QueryRowContext(ctx,
		`SELECT `+proxyReadColumns+` FROM proxies WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ProxyRecord{}, ErrNotFound
	}
	if err != nil {
		return model.ProxyRecord{}, fmt.Errorf("query proxy: %w", err)
	}
	return proxy, nil
}

func (s *Store) CreateProxy(ctx context.Context, proxy model.ProxyRecord) (model.ProxyRecord, error) {
	return s.CreateProxyWithPassword(ctx, proxy, nil)
}

func (s *Store) UpdateProxy(ctx context.Context, proxy model.ProxyRecord) (model.ProxyRecord, error) {
	return s.UpdateProxyWithPassword(ctx, proxy, nil, false)
}

func (s *Store) DeleteProxy(ctx context.Context, id string) (int, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin delete proxy: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		`SELECT id, encrypted_payload, password_source FROM hosts
		 WHERE proxy_id = ? AND encrypted_payload IS NOT NULL`, id)
	if err != nil {
		return 0, fmt.Errorf("query hosts by proxy: %w", err)
	}
	type referencedHost struct {
		id             string
		payload        []byte
		passwordSource string
	}
	var withPayload []referencedHost
	for rows.Next() {
		var host referencedHost
		if err := rows.Scan(&host.id, &host.payload, &host.passwordSource); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan referenced host: %w", err)
		}
		withPayload = append(withPayload, host)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterate referenced hosts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close referenced hosts: %w", err)
	}
	for _, host := range withPayload {
		if s.codec == nil || len(host.payload) == 0 {
			clear(host.payload)
			continue
		}
		var record model.Host
		record.ID = host.id

		record.PasswordSource = host.passwordSource
		if err := s.openHostPayload(host.id, host.payload, &record); err != nil {
			clear(host.payload)
			return 0, err
		}
		password, proxyPassword, err := s.openHostPasswords(host.id, host.payload)
		clear(host.payload)
		if err != nil {
			return 0, err
		}
		if proxyPassword == "" {
			continue
		}
		sealed, err := s.sealHostPayloadWithPasswords(record, password, "")
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE hosts SET encrypted_payload = ? WHERE id = ?`, sealed, host.id); err != nil {
			clear(sealed)
			return 0, fmt.Errorf("clear host proxy password: %w", err)
		}
		clear(sealed)
	}

	cleared, err := tx.ExecContext(ctx,
		`UPDATE hosts SET proxy_id = NULL, proxy_mode = 'none', proxy_host = '', proxy_port = 0, proxy_username = ''
		 WHERE proxy_id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("clear host proxy reference: %w", err)
	}
	referenced, err := cleared.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read cleared host count: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM proxies WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("delete proxy: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted proxy count: %w", err)
	}
	if count == 0 {
		return 0, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete proxy: %w", err)
	}
	return int(referenced), nil
}

func (s *Store) CountHostsByProxyID(ctx context.Context, id string) (int, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM hosts WHERE proxy_id = ?`, id).Scan(&count); err != nil {
		return 0, fmt.Errorf("count hosts by proxy: %w", err)
	}
	return count, nil
}

func normalizeProxyRecordColumns(proxy model.ProxyRecord) model.ProxyRecord {
	proxy.ID = strings.TrimSpace(proxy.ID)
	proxy.Name = strings.TrimSpace(proxy.Name)
	proxy.Mode = strings.TrimSpace(proxy.Mode)
	proxy.Host = strings.TrimSpace(proxy.Host)
	proxy.Username = strings.TrimSpace(proxy.Username)
	return proxy
}
