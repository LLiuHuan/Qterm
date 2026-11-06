package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"changeme/internal/model"
)

type encryptedProxyPayload struct {
	Password string `json:"password,omitempty"`
}

func (s *Store) CreateProxyWithPassword(ctx context.Context, proxy model.ProxyRecord, password []byte) (model.ProxyRecord, error) {
	proxy = normalizeProxyRecordColumns(proxy)
	encryptedPayload, err := s.sealProxyPayload(proxy.ID, password)
	if err != nil {
		return model.ProxyRecord{}, err
	}
	defer clear(encryptedPayload)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ProxyRecord{}, fmt.Errorf("begin create proxy: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO proxies (`+proxyColumns+`, encrypted_payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proxy.ID, proxy.Name, proxy.Mode, proxy.Host, proxy.Port,
		proxy.Username, proxy.CreatedAt, proxy.UpdatedAt,
		encryptedPayload,
	); err != nil {
		return model.ProxyRecord{}, fmt.Errorf("insert proxy: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.ProxyRecord{}, fmt.Errorf("commit proxy: %w", err)
	}
	proxy.PasswordConfigured = len(password) > 0
	return proxy, nil
}

func (s *Store) UpdateProxyWithPassword(
	ctx context.Context,
	proxy model.ProxyRecord,
	password []byte,
	clearPassword bool,
) (model.ProxyRecord, error) {
	proxy = normalizeProxyRecordColumns(proxy)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ProxyRecord{}, fmt.Errorf("begin update proxy: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingCreatedAt string
	var existingPayload []byte
	if err := tx.QueryRowContext(ctx,
		`SELECT created_at, encrypted_payload FROM proxies WHERE id = ?`, proxy.ID).Scan(&existingCreatedAt, &existingPayload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ProxyRecord{}, ErrNotFound
		}
		return model.ProxyRecord{}, fmt.Errorf("read existing proxy password: %w", err)
	}

	storedPassword := append([]byte(nil), password...)

	defer func() { clear(storedPassword) }()
	if len(storedPassword) == 0 && !clearPassword {
		existingPassword, openErr := s.openProxyPassword(proxy.ID, existingPayload)
		clear(existingPayload)
		if openErr != nil {
			return model.ProxyRecord{}, openErr
		}
		storedPassword = []byte(existingPassword)
	} else {
		clear(existingPayload)
	}

	encryptedPayload, err := s.sealProxyPayload(proxy.ID, storedPassword)
	if err != nil {
		return model.ProxyRecord{}, err
	}
	defer clear(encryptedPayload)

	result, err := tx.ExecContext(ctx,
		`UPDATE proxies
		 SET name = ?, mode = ?, host = ?, port = ?, username = ?, updated_at = ?, encrypted_payload = ?
		 WHERE id = ?`,
		proxy.Name, proxy.Mode, proxy.Host, proxy.Port,
		proxy.Username, proxy.UpdatedAt, encryptedPayload, proxy.ID,
	)
	if err != nil {
		return model.ProxyRecord{}, fmt.Errorf("update proxy: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return model.ProxyRecord{}, fmt.Errorf("read updated proxy count: %w", err)
	}
	if count == 0 {
		return model.ProxyRecord{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return model.ProxyRecord{}, fmt.Errorf("commit proxy update: %w", err)
	}
	proxy.CreatedAt = existingCreatedAt
	proxy.PasswordConfigured = len(storedPassword) > 0
	return proxy, nil
}

func (s *Store) UseProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || s.codec == nil || use == nil {
		return model.ErrCredentialVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT encrypted_payload FROM proxies WHERE id = ?`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read proxy password: %w", err)
	}
	password, err := s.openProxyPassword(id, payload)
	clear(payload)
	if err != nil {
		return err
	}
	if password == "" {
		return ErrNotFound
	}
	secret := []byte(password)
	defer clear(secret)
	return use(secret)
}

func (s *Store) sealProxyPayload(id string, password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, nil
	}
	if s == nil || s.codec == nil {
		return nil, model.ErrCredentialVaultLocked
	}
	plaintext, err := json.Marshal(encryptedProxyPayload{Password: string(password)})
	if err != nil {
		return nil, fmt.Errorf("encode proxy payload: %w", err)
	}
	defer clear(plaintext)
	sealed, err := s.codec.Seal("proxy", id, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt proxy payload: %w", err)
	}
	return sealed, nil
}

func (s *Store) openProxyPassword(id string, payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", nil
	}
	if s == nil || s.codec == nil {
		return "", model.ErrCredentialVaultLocked
	}
	plaintext, err := s.codec.Open("proxy", id, payload)
	if err != nil {
		return "", fmt.Errorf("decrypt proxy payload: %w", err)
	}
	defer clear(plaintext)
	var decoded encryptedProxyPayload
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return "", fmt.Errorf("decode proxy payload: %w", err)
	}
	return decoded.Password, nil
}
