package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"changeme/internal/model"
)

func (s *Store) repairProxyNamesAfterBackfill(ctx context.Context, tx *sql.Tx) error {
	if s == nil || s.codec == nil {
		return nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, encrypted_payload, proxy_id
		 FROM hosts
		 WHERE proxy_id IS NOT NULL AND encrypted_payload IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("query hosts for proxy name repair: %w", err)
	}
	type proxyNameRepair struct {
		proxyID string
		name    string
	}
	repairs := make([]proxyNameRepair, 0)
	for rows.Next() {
		var (
			hostID  string
			payload []byte
			proxyID string
		)
		if err := rows.Scan(&hostID, &payload, &proxyID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan host for proxy name repair: %w", err)
		}
		if strings.TrimSpace(proxyID) == "" || len(payload) == 0 {
			continue
		}
		var host model.Host

		if err := s.openHostPayload(hostID, payload, &host); err != nil {
			_ = rows.Close()
			return err
		}
		name := strings.TrimSpace(host.Name)
		if name == "" {
			name = strings.TrimSpace(host.Address)
		}
		if name == "" {

			continue
		}
		repairs = append(repairs, proxyNameRepair{proxyID: strings.TrimSpace(proxyID), name: name + " 代理"})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate hosts for proxy name repair: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close hosts for proxy name repair: %w", err)
	}

	for _, repair := range repairs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE proxies SET name = ? WHERE id = ? AND name = ?`,
			repair.name, repair.proxyID, " 代理"); err != nil {
			return fmt.Errorf("repair proxy name: %w", err)
		}
	}
	return nil
}
