package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"changeme/internal/model"
)

func (s *Store) UseHostPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || s.codec == nil || use == nil {
		return model.ErrCredentialVaultLocked
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ?`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read host password: %w", err)
	}
	password, err := s.openHostPassword(id, payload)
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

func (s *Store) RememberHostPassword(ctx context.Context, id string, password []byte) error {
	if s == nil || s.codec == nil {
		return model.ErrCredentialVaultLocked
	}
	if len(password) == 0 {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remember host password: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var payload []byte
	if err := tx.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ? AND password_source = 'prompt'`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read host for password update: %w", err)
	}
	host := model.Host{ID: id}
	if err := s.openHostPayload(id, payload, &host); err != nil {
		clear(payload)
		return err
	}
	proxyPassword, err := s.openHostProxyPassword(id, payload)
	clear(payload)
	if err != nil {
		return err
	}
	defer clear([]byte(proxyPassword))
	if host.PasswordSource != "prompt" || host.CredentialID != "" {
		return ErrNotFound
	}
	host.PasswordSource = "saved"
	sealed, err := s.sealHostPayloadWithPasswords(host, string(password), proxyPassword)
	if err != nil {
		return err
	}
	defer clear(sealed)
	if _, err := tx.ExecContext(ctx, `UPDATE hosts SET encrypted_payload = ?, password_source = 'saved' WHERE id = ? AND password_source = 'prompt'`, sealed, id); err != nil {
		return fmt.Errorf("save remembered host password: %w", err)
	}
	return tx.Commit()
}

func (s *Store) RememberHostCredential(ctx context.Context, id, credentialID string) error {
	credentialID = strings.TrimSpace(credentialID)
	if s == nil || credentialID == "" {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remember host credential: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if s.codec == nil {
		result, updateErr := tx.ExecContext(ctx, `UPDATE hosts SET credential_id = ?, password_source = 'none', updated_at = ?
			WHERE id = ? AND password_source = 'prompt' AND credential_id = ''`, credentialID, formatTime(time.Now()), id)
		if updateErr != nil {
			return fmt.Errorf("save remembered host credential: %w", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return ErrNotFound
		}
		return tx.Commit()
	}

	var payload []byte
	if err := tx.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ? AND password_source = 'prompt'`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read host for credential update: %w", err)
	}
	host := model.Host{ID: id, PasswordSource: "prompt"}
	if err := s.openHostPayload(id, payload, &host); err != nil {
		clear(payload)
		return err
	}
	proxyPassword, err := s.openHostProxyPassword(id, payload)
	clear(payload)
	if err != nil {
		return err
	}
	defer clear([]byte(proxyPassword))
	if host.PasswordSource != "prompt" || host.CredentialID != "" {
		return ErrNotFound
	}
	host.CredentialID = credentialID
	host.PasswordSource = "none"
	sealed, err := s.sealHostPayloadWithPasswords(host, "", proxyPassword)
	if err != nil {
		return err
	}
	defer clear(sealed)
	result, err := tx.ExecContext(ctx, `UPDATE hosts SET encrypted_payload = ?, credential_id = '', password_source = 'none', updated_at = ?
		WHERE id = ? AND password_source = 'prompt'`, sealed, formatTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("save remembered host credential: %w", err)
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil || rows != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

type encryptedHostPayload struct {
	Icon             string `json:"icon"`
	Name             string `json:"name"`
	Protocol         string `json:"protocol"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	CredentialID     string `json:"credentialId"`
	PasswordSource   string `json:"passwordSource,omitempty"`
	Notes            string `json:"notes"`
	SSHKeepAlive     *bool  `json:"sshKeepAlive,omitempty"`
	LatencyDetection *bool  `json:"latencyDetection,omitempty"`
	AutoDetectOSIcon *bool  `json:"autoDetectOSIcon,omitempty"`
	DirectoryFollow  string `json:"directoryFollow"`

	SerialBaud     int    `json:"serialBaud,omitempty"`
	SerialDataBits int    `json:"serialDataBits,omitempty"`
	SerialParity   string `json:"serialParity,omitempty"`
	SerialStopBits string `json:"serialStopBits,omitempty"`
	Password       string `json:"password,omitempty"`
	ProxyPassword  string `json:"proxyPassword,omitempty"`
}

func (s *Store) migrateEncryptedHostPayloads(ctx context.Context) error {
	if s == nil || s.codec == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, icon, name, protocol, address, port, username, credential_id, notes, encrypted_payload,
		ssh_keep_alive, latency_detection, auto_detect_os_icon, directory_follow, password_source FROM hosts`)
	if err != nil {
		return fmt.Errorf("inspect host encryption payloads: %w", err)
	}
	defer rows.Close()
	type migration struct {
		id      string
		payload []byte
	}
	var pending []migration
	for rows.Next() {
		var id, icon, name, protocol, address, username, credentialID, notes string
		var port int
		var payload []byte
		var keepAlive, latency, autoIcon bool
		var follow, passwordSource string
		if err := rows.Scan(&id, &icon, &name, &protocol, &address, &port, &username, &credentialID, &notes, &payload,
			&keepAlive, &latency, &autoIcon, &follow, &passwordSource); err != nil {
			return fmt.Errorf("scan host encryption payload: %w", err)
		}
		if len(payload) != 0 {
			continue
		}
		plaintext, err := json.Marshal(encryptedHostPayload{Icon: icon, Name: name, Protocol: protocol, Address: address, Port: port, Username: username, CredentialID: credentialID, PasswordSource: passwordSource, Notes: notes,
			SSHKeepAlive: &keepAlive, LatencyDetection: &latency, AutoDetectOSIcon: &autoIcon, DirectoryFollow: follow})
		if err != nil {
			return fmt.Errorf("encode host encryption payload: %w", err)
		}
		sealed, err := s.codec.Seal("host", id, plaintext)
		clear(plaintext)
		if err != nil {
			return fmt.Errorf("encrypt host payload: %w", err)
		}
		pending = append(pending, migration{id: id, payload: sealed})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate host encryption payloads: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin host encryption migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range pending {
		if _, err := tx.ExecContext(ctx, `UPDATE hosts SET encrypted_payload = ?, icon = '', name = '', protocol = 'ssh', address = '', port = 22, username = '', credential_id = '', notes = '',
			ssh_keep_alive = 1, latency_detection = 1, auto_detect_os_icon = 1, directory_follow = 'inherit' WHERE id = ? AND encrypted_payload IS NULL`, item.payload, item.id); err != nil {
			return fmt.Errorf("store encrypted host payload: %w", err)
		}
		clear(item.payload)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit host encryption migration: %w", err)
	}
	return nil
}

func (s *Store) sealHostPayload(host model.Host) ([]byte, error) {
	return s.sealHostPayloadWithPasswords(host, "", "")
}

func (s *Store) sealHostPayloadWithPassword(host model.Host, password string) ([]byte, error) {
	return s.sealHostPayloadWithPasswords(host, password, "")
}

func (s *Store) sealHostPayloadWithPasswords(host model.Host, password, proxyPassword string) ([]byte, error) {
	if s.codec == nil {
		return nil, nil
	}
	plaintext, err := json.Marshal(encryptedHostPayload{
		Icon: host.Icon, Name: host.Name, Protocol: host.Protocol, Address: host.Address,
		Port: host.Port, Username: host.Username, CredentialID: host.CredentialID, PasswordSource: host.PasswordSource, Notes: host.Notes,
		SSHKeepAlive: boolPtr(host.SSHKeepAlive), LatencyDetection: boolPtr(host.LatencyDetection),
		AutoDetectOSIcon: boolPtr(host.AutoDetectOSIcon), DirectoryFollow: host.DirectoryFollow,
		SerialBaud: host.SerialBaud, SerialDataBits: host.SerialDataBits,
		SerialParity: host.SerialParity, SerialStopBits: host.SerialStopBits,
		Password:      password,
		ProxyPassword: proxyPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("encode host payload: %w", err)
	}
	defer clear(plaintext)
	return s.codec.Seal("host", host.ID, plaintext)
}

func (s *Store) openHostPayload(id string, payload []byte, host *model.Host) error {
	if s.codec == nil || len(payload) == 0 {
		return nil
	}
	plaintext, err := s.codec.Open("host", id, payload)
	if err != nil {
		return fmt.Errorf("decrypt host payload: %w", err)
	}
	defer clear(plaintext)
	var decoded encryptedHostPayload
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return fmt.Errorf("decode host payload: %w", err)
	}
	host.Icon = decoded.Icon
	host.Name = decoded.Name
	host.Protocol = decoded.Protocol
	host.Address = decoded.Address
	host.Port = decoded.Port
	host.Username = decoded.Username
	host.CredentialID = decoded.CredentialID

	storedPasswordSource := host.PasswordSource
	if storedPasswordSource == "" {
		storedPasswordSource = decoded.PasswordSource
	}
	host.PasswordSource = storedPasswordSource
	host.Notes = decoded.Notes
	host.SSHKeepAlive = boolValue(decoded.SSHKeepAlive, true)
	host.LatencyDetection = boolValue(decoded.LatencyDetection, true)
	host.AutoDetectOSIcon = boolValue(decoded.AutoDetectOSIcon, true)
	host.DirectoryFollow = decoded.DirectoryFollow
	if host.DirectoryFollow == "" {
		host.DirectoryFollow = "inherit"
	}
	if decoded.SerialBaud > 0 {
		host.SerialBaud = decoded.SerialBaud
	}
	if decoded.SerialDataBits > 0 {
		host.SerialDataBits = decoded.SerialDataBits
	}
	if decoded.SerialParity != "" {
		host.SerialParity = decoded.SerialParity
	}
	if decoded.SerialStopBits != "" {
		host.SerialStopBits = decoded.SerialStopBits
	}
	host.PasswordConfigured = decoded.Password != ""
	host.ProxyPasswordConfigured = decoded.ProxyPassword != ""
	if host.PasswordSource == "" {
		if host.PasswordConfigured {
			host.PasswordSource = "saved"
		} else {
			host.PasswordSource = "none"
		}
	} else if host.PasswordSource == "none" && host.PasswordConfigured && decoded.PasswordSource == "" {

		host.PasswordSource = "saved"
	}
	return nil
}

func boolPtr(value bool) *bool { return &value }

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *Store) openHostPassword(id string, payload []byte) (string, error) {
	password, _, err := s.openHostPasswords(id, payload)
	return password, err
}

func (s *Store) openHostProxyPassword(id string, payload []byte) (string, error) {
	_, password, err := s.openHostPasswords(id, payload)
	return password, err
}

func (s *Store) openHostPasswords(id string, payload []byte) (string, string, error) {
	if s.codec == nil || len(payload) == 0 {
		return "", "", nil
	}
	plaintext, err := s.codec.Open("host", id, payload)
	if err != nil {
		return "", "", fmt.Errorf("decrypt host password: %w", err)
	}
	defer clear(plaintext)
	var decoded encryptedHostPayload
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return "", "", fmt.Errorf("decode host password: %w", err)
	}
	return decoded.Password, decoded.ProxyPassword, nil
}

func (s *Store) UseHostProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || s.codec == nil || use == nil {
		return model.ErrCredentialVaultLocked
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ?`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read host proxy password: %w", err)
	}
	password, err := s.openHostProxyPassword(id, payload)
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

func sanitizeHostForStorage(host model.Host, payload []byte) model.Host {
	if len(payload) == 0 {
		return host
	}
	host.Icon = ""
	host.Name = ""
	host.Protocol = "ssh"
	host.Address = ""
	host.Port = 22
	host.Username = ""
	host.CredentialID = ""
	host.Notes = ""
	return host
}
