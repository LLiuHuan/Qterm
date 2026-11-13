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

	"modernc.org/sqlite"
)

func (s *Store) ListHosts(ctx context.Context) ([]model.Host, error) {
	const query = `
		SELECT h.id, h.icon, h.color, h.name, h.protocol, h.address, h.port, h.username,
		       g.id, g.name, h.sort_order, h.credential_id, h.notes, h.created_at, h.updated_at, h.encrypted_payload,
		       h.ssh_keep_alive, h.latency_detection, h.auto_detect_os_icon, h.directory_follow, h.password_source,
		       h.ssh_two_factor_enabled, h.ssh_otp_entry_id, h.ssh_otp_auto_fill,
		       h.sftp_show_hidden_files, h.sftp_default_directory,
		       h.backspace_mode, h.encoding, h.connect_timeout_seconds, h.post_login_command, h.environment_variables,
		       h.x11_forwarding, h.x11_trusted, h.rdp_display_mode, h.rdp_width, h.rdp_height, h.post_login_command_delay_seconds,
		       h.rdp_clipboard_mode, h.rdp_clipboard_content, h.tags,
		       h.vnc_view_only, h.vnc_shared, h.vnc_pixel_depth,
		       h.vnc_scale, h.vnc_auto_reconnect, h.vnc_auto_reconnect_attempts,
		       h.vnc_transport, h.vnc_ws_path, h.vnc_allow_insecure_tls, h.vnc_rfb_password_mode,
		       h.connection_type, h.terminal_type,
		       h.jump_host_id, h.proxy_mode, h.proxy_host, h.proxy_port, h.proxy_username,
		       h.proxy_id
		FROM hosts h
		LEFT JOIN groups g ON g.id = h.group_id
		ORDER BY CASE WHEN h.group_id IS NULL THEN -1 ELSE g.sort_order END,
		         COALESCE(g.name, '') COLLATE NOCASE, h.sort_order, h.id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query hosts: %w", err)
	}
	defer rows.Close()

	hosts := make([]model.Host, 0)
	for rows.Next() {
		var (
			host                     model.Host
			groupID, groupName       sql.NullString
			createdAt, updatedAt     string
			payload                  []byte
			passwordSource           string
			environmentVariablesJSON string
			tagsJSON                 string
			jumpHostID               sql.NullString
			proxyMode                string
			proxyHost                string
			proxyPort                int
			proxyUsername            string
			proxyID                  sql.NullString
		)
		if err := rows.Scan(
			&host.ID,
			&host.Icon,
			&host.Color,
			&host.Name,
			&host.Protocol,
			&host.Address,
			&host.Port,
			&host.Username,
			&groupID,
			&groupName,
			&host.SortOrder,
			&host.CredentialID,
			&host.Notes,
			&createdAt,
			&updatedAt,
			&payload,
			&host.SSHKeepAlive,
			&host.LatencyDetection,
			&host.AutoDetectOSIcon,
			&host.DirectoryFollow,
			&passwordSource,
			&host.SSHTwoFactorEnabled,
			&host.SSHOTPEntryID,
			&host.SSHOTPAutoFill,
			&host.SFTPShowHiddenFiles,
			&host.SFTPDefaultDirectory,
			&host.BackspaceMode,
			&host.Encoding,
			&host.ConnectTimeoutSeconds,
			&host.PostLoginCommand,
			&environmentVariablesJSON,
			&host.X11Forwarding,
			&host.X11Trusted,
			&host.RdpDisplayMode,
			&host.RdpWidth,
			&host.RdpHeight,
			&host.PostLoginCommandDelaySeconds,
			&host.RdpClipboardMode,
			&host.RdpClipboardContent,
			&tagsJSON,
			&host.VncViewOnly,
			&host.VncShared,
			&host.VncPixelDepth,

			&host.VncScale,
			&host.VncAutoReconnect,
			&host.VncAutoReconnectAttempts,
			&host.VncTransport,
			&host.VncWsPath,
			&host.VncAllowInsecureTLS,

			&host.VncRfbPasswordMode,
			&host.ConnectionType,
			&host.TerminalType,

			&jumpHostID,
			&proxyMode,
			&proxyHost,
			&proxyPort,
			&proxyUsername,

			&proxyID,
		); err != nil {
			return nil, fmt.Errorf("scan host: %w", err)
		}
		applyHostProxy(jumpHostID, proxyMode, proxyHost, proxyPort, proxyUsername, proxyID, &host)
		host.GroupID = groupID.String
		host.Group = groupName.String
		host.PasswordSource = passwordSource
		if err := decodeEnvironmentVariables(environmentVariablesJSON, &host); err != nil {
			return nil, err
		}
		if err := decodeHostTags(tagsJSON, &host); err != nil {
			return nil, err
		}
		if err := s.openHostPayload(host.ID, payload, &host); err != nil {
			return nil, err
		}
		normalizeSSHOTPSettings(&host)
		host.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse host created time: %w", err)
		}
		host.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse host updated time: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hosts: %w", err)
	}
	return hosts, nil
}

func (s *Store) GetHost(ctx context.Context, id string) (model.Host, error) {
	const query = `
		SELECT h.id, h.icon, h.color, h.name, h.protocol, h.address, h.port, h.username,
		       g.id, g.name, h.sort_order, h.credential_id, h.notes, h.created_at, h.updated_at, h.encrypted_payload,
		       h.ssh_keep_alive, h.latency_detection, h.auto_detect_os_icon, h.directory_follow, h.password_source,
		       h.ssh_two_factor_enabled, h.ssh_otp_entry_id, h.ssh_otp_auto_fill,
		       h.sftp_show_hidden_files, h.sftp_default_directory,
		       h.backspace_mode, h.encoding, h.connect_timeout_seconds, h.post_login_command, h.environment_variables,
		       h.x11_forwarding, h.x11_trusted, h.rdp_display_mode, h.rdp_width, h.rdp_height, h.post_login_command_delay_seconds,
		       h.rdp_clipboard_mode, h.rdp_clipboard_content, h.tags,
		       h.vnc_view_only, h.vnc_shared, h.vnc_pixel_depth,
		       h.vnc_scale, h.vnc_auto_reconnect, h.vnc_auto_reconnect_attempts,
		       h.vnc_transport, h.vnc_ws_path, h.vnc_allow_insecure_tls, h.vnc_rfb_password_mode,
		       h.connection_type, h.terminal_type,
		       h.jump_host_id, h.proxy_mode, h.proxy_host, h.proxy_port, h.proxy_username,
		       h.proxy_id
		FROM hosts h
		LEFT JOIN groups g ON g.id = h.group_id
		WHERE h.id = ?`

	var (
		host                     model.Host
		groupID, groupName       sql.NullString
		createdAt, updatedAt     string
		payload                  []byte
		environmentVariablesJSON string
		tagsJSON                 string
		jumpHostID               sql.NullString
		proxyMode                string
		proxyHost                string
		proxyPort                int
		proxyUsername            string
		proxyID                  sql.NullString
	)
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&host.ID,
		&host.Icon,
		&host.Color,
		&host.Name,
		&host.Protocol,
		&host.Address,
		&host.Port,
		&host.Username,
		&groupID,
		&groupName,
		&host.SortOrder,
		&host.CredentialID,
		&host.Notes,
		&createdAt,
		&updatedAt,
		&payload,
		&host.SSHKeepAlive,
		&host.LatencyDetection,
		&host.AutoDetectOSIcon,
		&host.DirectoryFollow,
		&host.PasswordSource,
		&host.SSHTwoFactorEnabled,
		&host.SSHOTPEntryID,
		&host.SSHOTPAutoFill,
		&host.SFTPShowHiddenFiles,
		&host.SFTPDefaultDirectory,
		&host.BackspaceMode,
		&host.Encoding,
		&host.ConnectTimeoutSeconds,
		&host.PostLoginCommand,
		&environmentVariablesJSON,
		&host.X11Forwarding,
		&host.X11Trusted,
		&host.RdpDisplayMode,
		&host.RdpWidth,
		&host.RdpHeight,
		&host.PostLoginCommandDelaySeconds,
		&host.RdpClipboardMode,
		&host.RdpClipboardContent,
		&tagsJSON,
		&host.VncViewOnly,
		&host.VncShared,
		&host.VncPixelDepth,

		&host.VncScale,
		&host.VncAutoReconnect,
		&host.VncAutoReconnectAttempts,
		&host.VncTransport,
		&host.VncWsPath,
		&host.VncAllowInsecureTLS,
		&host.VncRfbPasswordMode,
		&host.ConnectionType,
		&host.TerminalType,

		&jumpHostID,
		&proxyMode,
		&proxyHost,
		&proxyPort,
		&proxyUsername,

		&proxyID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Host{}, ErrNotFound
	}
	if err != nil {
		return model.Host{}, fmt.Errorf("query host: %w", err)
	}
	applyHostProxy(jumpHostID, proxyMode, proxyHost, proxyPort, proxyUsername, proxyID, &host)
	host.GroupID = groupID.String
	host.Group = groupName.String
	if err := decodeEnvironmentVariables(environmentVariablesJSON, &host); err != nil {
		return model.Host{}, err
	}
	if err := decodeHostTags(tagsJSON, &host); err != nil {
		return model.Host{}, err
	}
	if err := s.openHostPayload(host.ID, payload, &host); err != nil {
		return model.Host{}, err
	}
	normalizeSSHOTPSettings(&host)
	host.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return model.Host{}, fmt.Errorf("parse host created time: %w", err)
	}
	host.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return model.Host{}, fmt.Errorf("parse host updated time: %w", err)
	}
	return host, nil
}

func (s *Store) SetHostIcon(ctx context.Context, id string, icon string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(icon) == "" {
		return errors.New("host icon update requires identifiers")
	}
	if s.codec != nil {
		host, err := s.GetHost(ctx, id)
		if err != nil {
			return err
		}
		host.Icon = icon
		var existingPayload []byte
		if err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ?`, id).Scan(&existingPayload); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read host icon payload: %w", err)
		}
		password, err := s.openHostPassword(id, existingPayload)
		proxyPassword, proxyErr := s.openHostProxyPassword(id, existingPayload)
		clear(existingPayload)
		if err != nil {
			return err
		}
		if proxyErr != nil {
			clear([]byte(password))
			return proxyErr
		}
		payload, err := s.sealHostPayloadWithPasswords(host, password, proxyPassword)
		clear([]byte(password))
		clear([]byte(proxyPassword))
		if err != nil {
			return err
		}
		defer clear(payload)
		_, err = s.db.ExecContext(ctx, `UPDATE hosts SET encrypted_payload = ?, updated_at = ? WHERE id = ?`, payload, formatTime(time.Now()), id)
		return err
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE hosts SET icon = ?, updated_at = ? WHERE id = ? AND icon = ''`,
		icon, formatTime(time.Now()), id,
	)
	if err != nil {
		return fmt.Errorf("update detected host icon: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("read detected host icon update: %w", err)
	} else if count == 0 {
		return nil
	}
	return nil
}

func normalizeShowHiddenFiles(value string) string {
	if value == "" {
		return "inherit"
	}
	return value
}

func normalizeBackspaceMode(value string) string {
	if value != "bs" {
		return "del"
	}
	return value
}

func normalizeEncoding(value string) string {
	if value != "gbk" {
		return "utf-8"
	}
	return value
}

func normalizeConnectionType(value string) string {
	if value != model.ConnectionTypeNetwork {
		return model.ConnectionTypeStandard
	}
	return value
}

func normalizeTerminalType(connectionType, terminalType string) string {
	if model.ValidTerminalType(terminalType) {
		return terminalType
	}
	return model.DefaultTerminalType(normalizeConnectionType(connectionType))
}

func normalizePostLoginDelay(value int) int {
	if value < 0 || value > model.MaxPostLoginCommandDelaySeconds {
		return 0
	}
	return value
}

func normalizeConnectTimeout(value int) int {
	if value < model.ConnectTimeoutSecondsMin || value > model.ConnectTimeoutSecondsMax {
		return model.DefaultConnectTimeoutSeconds
	}
	return value
}

func normalizeEnvironmentVariablesJSON(values []model.HostEnvironmentVariable) string {
	if len(values) == 0 {
		return ""
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func decodeEnvironmentVariables(raw string, host *model.Host) error {
	if strings.TrimSpace(raw) == "" {
		host.EnvironmentVariables = nil
		return nil
	}
	var values []model.HostEnvironmentVariable
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return fmt.Errorf("decode host environment variables: %w", err)
	}
	host.EnvironmentVariables = values
	return nil
}

func (s *Store) CreateHost(ctx context.Context, host model.Host, group model.Group) (model.Host, error) {
	return s.CreateHostWithPasswords(ctx, host, group, nil, nil)
}

func (s *Store) CreateHostWithPassword(ctx context.Context, host model.Host, group model.Group, password []byte) (model.Host, error) {
	return s.CreateHostWithPasswords(ctx, host, group, password, nil)
}

func (s *Store) CreateHostWithPasswords(ctx context.Context, host model.Host, group model.Group, password, proxyPassword []byte) (model.Host, error) {
	if len(proxyPassword) > 0 && (s == nil || s.codec == nil) {
		return model.Host{}, model.ErrCredentialVaultLocked
	}
	if host.DirectoryFollow == "" {
		host.DirectoryFollow = "inherit"
	}
	host.SFTPShowHiddenFiles = normalizeShowHiddenFiles(host.SFTPShowHiddenFiles)
	normalizeSSHOTPSettings(&host)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Host{}, fmt.Errorf("begin create host: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	groupID, groupName := "", ""
	if group.Name != "" {
		groupID, groupName, err = ensureGroup(ctx, tx, group)
		if err != nil {
			return model.Host{}, err
		}
	}
	host.GroupID = groupID
	host.Group = groupName
	resultHost := host
	var encryptedPayload []byte
	if s.codec != nil {
		encryptedPayload, err = s.sealHostPayloadWithPasswords(host, string(password), string(proxyPassword))
		if err != nil {
			return model.Host{}, err
		}
		defer clear(encryptedPayload)
		host = sanitizeHostForStorage(host, encryptedPayload)
		resultHost.PasswordConfigured = len(password) > 0
		resultHost.ProxyPasswordConfigured = len(proxyPassword) > 0
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM (SELECT sort_order FROM groups WHERE parent_id IS ? UNION ALL SELECT sort_order FROM hosts WHERE group_id IS ?)`, nullableID(groupID), nullableID(groupID)).Scan(&host.SortOrder); err != nil {
		return model.Host{}, fmt.Errorf("next host sort order: %w", err)
	}
	resultHost.SortOrder = host.SortOrder

	proxyMode, proxyHost, proxyPort, proxyUsername := normalizeHostProxyColumns(
		textValue(host.ProxyMode), textValue(host.ProxyHost), intValue(host.ProxyPort),
		textValue(host.ProxyUsername),
	)

	const insertHost = `
		INSERT INTO hosts (
			id, icon, color, name, protocol, address, port, username, group_id, sort_order, credential_id, notes, created_at, updated_at, encrypted_payload,
			ssh_keep_alive, latency_detection, auto_detect_os_icon, directory_follow, password_source,
			ssh_two_factor_enabled, ssh_otp_entry_id, ssh_otp_auto_fill,
			sftp_show_hidden_files, sftp_default_directory,
			backspace_mode, encoding, connect_timeout_seconds, post_login_command, environment_variables,
			x11_forwarding, x11_trusted, rdp_display_mode, rdp_width, rdp_height, post_login_command_delay_seconds,
			rdp_clipboard_mode, rdp_clipboard_content, tags,
			vnc_view_only, vnc_shared, vnc_pixel_depth,
			vnc_scale, vnc_auto_reconnect, vnc_auto_reconnect_attempts,
			vnc_transport, vnc_ws_path, vnc_allow_insecure_tls, vnc_rfb_password_mode,
			connection_type, terminal_type,
			jump_host_id, proxy_mode, proxy_host, proxy_port, proxy_username, proxy_id
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''),
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?
		)`
	if _, err := tx.ExecContext(
		ctx,
		insertHost,
		host.ID,
		host.Icon,
		host.Color,
		host.Name,
		host.Protocol,
		host.Address,
		host.Port,
		host.Username,
		host.GroupID,
		host.SortOrder,
		host.CredentialID,
		host.Notes,
		formatTime(host.CreatedAt),
		formatTime(host.UpdatedAt),
		encryptedPayload,
		host.SSHKeepAlive, host.LatencyDetection, host.AutoDetectOSIcon, host.DirectoryFollow, host.PasswordSource,
		host.SSHTwoFactorEnabled, host.SSHOTPEntryID, host.SSHOTPAutoFill,
		normalizeShowHiddenFiles(host.SFTPShowHiddenFiles), host.SFTPDefaultDirectory,
		normalizeBackspaceMode(host.BackspaceMode), normalizeEncoding(host.Encoding),
		normalizeConnectTimeout(host.ConnectTimeoutSeconds), host.PostLoginCommand,
		normalizeEnvironmentVariablesJSON(host.EnvironmentVariables), host.X11Forwarding, host.X11Trusted, normalizeRdpDisplayMode(host.RdpDisplayMode), normalizeRdpWidth(host.RdpWidth), normalizeRdpHeight(host.RdpHeight), normalizePostLoginDelay(host.PostLoginCommandDelaySeconds),
		normalizeRdpClipboardMode(host.RdpClipboardMode), normalizeRdpClipboardContent(host.RdpClipboardContent), normalizeHostTagsJSON(host.Tags),
		normalizeVncViewOnly(host.VncViewOnly), normalizeVncShared(host.VncShared), normalizeVncPixelDepth(host.VncPixelDepth),
		normalizeVncScale(host.VncScale), normalizeVncAutoReconnect(host.VncAutoReconnect), normalizeVncAutoReconnectAttempts(host.VncAutoReconnectAttempts),
		normalizeVncTransport(host.VncTransport), normalizeVncWsPath(host.VncWsPath), normalizeVncAllowInsecureTLS(host.VncAllowInsecureTLS),
		normalizeVncRfbPasswordMode(host.VncRfbPasswordMode),
		normalizeConnectionType(host.ConnectionType), normalizeTerminalType(host.ConnectionType, host.TerminalType),

		nullableString(textValue(host.JumpHostID)),
		proxyMode, proxyHost, proxyPort, proxyUsername,

		nullableString(host.ProxyID),
	); err != nil {
		return model.Host{}, fmt.Errorf("insert host: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Host{}, fmt.Errorf("commit host: %w", err)
	}

	return resultHost, nil
}

func (s *Store) DuplicateHost(ctx context.Context, id, newID, name string) (model.Host, error) {
	if s == nil || s.db == nil {
		return model.Host{}, ErrNotFound
	}
	original, err := s.GetHost(ctx, id)
	if err != nil {
		return model.Host{}, err
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ?`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Host{}, ErrNotFound
		}
		return model.Host{}, fmt.Errorf("read host payload for duplicate: %w", err)
	}
	password, proxyPassword, err := s.openHostPasswords(id, payload)
	clear(payload)
	if err != nil {
		return model.Host{}, err
	}

	now := time.Now()
	duplicate := original
	duplicate.ID = newID
	duplicate.Name = name
	duplicate.CreatedAt = now
	duplicate.UpdatedAt = now

	group := model.Group{Name: original.Group}
	created, err := s.CreateHostWithPasswords(ctx, duplicate, group, []byte(password), []byte(proxyPassword))
	if err != nil {
		return model.Host{}, err
	}
	return created, nil
}

func (s *Store) UpdateHost(ctx context.Context, host model.Host, group model.Group) (model.Host, error) {
	return s.UpdateHostWithPasswords(ctx, host, group, nil, false, nil, false)
}

func (s *Store) UpdateHostWithPassword(ctx context.Context, host model.Host, group model.Group, password []byte, clearPassword bool) (model.Host, error) {
	return s.UpdateHostWithPasswords(ctx, host, group, password, clearPassword, nil, false)
}

func (s *Store) UpdateHostWithPasswords(ctx context.Context, host model.Host, group model.Group, password []byte, clearPassword bool, proxyPassword []byte, clearProxyPassword bool) (model.Host, error) {
	if len(proxyPassword) > 0 && (s == nil || s.codec == nil) {
		return model.Host{}, model.ErrCredentialVaultLocked
	}
	if host.DirectoryFollow == "" {
		host.DirectoryFollow = "inherit"
	}
	host.SFTPShowHiddenFiles = normalizeShowHiddenFiles(host.SFTPShowHiddenFiles)
	normalizeSSHOTPSettings(&host)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Host{}, fmt.Errorf("begin update host: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	groupID, groupName := "", ""
	if group.Name != "" {
		groupID, groupName, err = ensureGroup(ctx, tx, group)
		if err != nil {
			return model.Host{}, err
		}
	}
	host.GroupID = groupID
	host.Group = groupName
	resultHost := host
	var currentGroupID sql.NullString
	var currentOrder int
	if err := tx.QueryRowContext(ctx, `SELECT group_id, sort_order FROM hosts WHERE id = ?`, host.ID).Scan(&currentGroupID, &currentOrder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Host{}, ErrNotFound
		}
		return model.Host{}, fmt.Errorf("read host order: %w", err)
	}
	if currentGroupID.String == groupID {
		host.SortOrder = currentOrder
	} else if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM (SELECT sort_order FROM groups WHERE parent_id IS ? UNION ALL SELECT sort_order FROM hosts WHERE group_id IS ?)`, nullableID(groupID), nullableID(groupID)).Scan(&host.SortOrder); err != nil {
		return model.Host{}, fmt.Errorf("next host sort order: %w", err)
	}
	resultHost.SortOrder = host.SortOrder
	var encryptedPayload []byte
	storedPassword := string(password)
	storedProxyPassword := string(proxyPassword)
	if s.codec != nil {
		if (storedPassword == "" && !clearPassword) || (storedProxyPassword == "" && !clearProxyPassword) {
			var existingPayload []byte
			if err := tx.QueryRowContext(ctx, `SELECT encrypted_payload FROM hosts WHERE id = ?`, host.ID).Scan(&existingPayload); err != nil {
				return model.Host{}, fmt.Errorf("read existing host password: %w", err)
			}
			var passwordErr error
			storedPasswordFromPayload, storedProxyPasswordFromPayload, passwordErr := s.openHostPasswords(host.ID, existingPayload)
			clear(existingPayload)
			if passwordErr != nil {
				return model.Host{}, passwordErr
			}
			if storedPassword == "" && !clearPassword {
				storedPassword = storedPasswordFromPayload
			}
			if storedProxyPassword == "" && !clearProxyPassword {
				storedProxyPassword = storedProxyPasswordFromPayload
			}
		}
		encryptedPayload, err = s.sealHostPayloadWithPasswords(host, storedPassword, storedProxyPassword)
		if err != nil {
			return model.Host{}, err
		}
		defer clear(encryptedPayload)
		host = sanitizeHostForStorage(host, encryptedPayload)
		resultHost.PasswordConfigured = storedPassword != ""
		resultHost.ProxyPasswordConfigured = storedProxyPassword != ""
	}

	const updateHost = `
		UPDATE hosts
		SET icon = ?, color = ?, name = ?, protocol = ?, address = ?, port = ?, username = ?, group_id = NULLIF(?, ''), sort_order = ?, credential_id = ?, notes = ?, updated_at = ?, encrypted_payload = ?,
		    ssh_keep_alive = ?, latency_detection = ?, auto_detect_os_icon = ?, directory_follow = ?, password_source = ?,
		    ssh_two_factor_enabled = ?, ssh_otp_entry_id = ?, ssh_otp_auto_fill = ?,
		    sftp_show_hidden_files = ?, sftp_default_directory = ?,
		    backspace_mode = ?, encoding = ?, connect_timeout_seconds = ?, post_login_command = ?, environment_variables = ?,
		    x11_forwarding = ?, x11_trusted = ?, rdp_display_mode = ?, rdp_width = ?, rdp_height = ?, post_login_command_delay_seconds = ?,
		    rdp_clipboard_mode = ?, rdp_clipboard_content = ?, tags = ?,
		    vnc_view_only = ?, vnc_shared = ?, vnc_pixel_depth = ?,
		    vnc_scale = ?, vnc_auto_reconnect = ?, vnc_auto_reconnect_attempts = ?,
		    vnc_transport = ?, vnc_ws_path = ?, vnc_allow_insecure_tls = ?, vnc_rfb_password_mode = ?,
		    connection_type = ?, terminal_type = ?,
		    jump_host_id = ?, proxy_mode = ?, proxy_host = ?, proxy_port = ?, proxy_username = ?, proxy_id = ?
		WHERE id = ?`
	proxyMode, proxyHost, proxyPort, proxyUsername := normalizeHostProxyColumns(
		textValue(host.ProxyMode), textValue(host.ProxyHost), intValue(host.ProxyPort),
		textValue(host.ProxyUsername),
	)
	result, err := tx.ExecContext(
		ctx,
		updateHost,
		host.Icon,
		host.Color,
		host.Name,
		host.Protocol,
		host.Address,
		host.Port,
		host.Username,
		host.GroupID,
		host.SortOrder,
		host.CredentialID,
		host.Notes,
		formatTime(host.UpdatedAt),
		encryptedPayload,
		host.SSHKeepAlive, host.LatencyDetection, host.AutoDetectOSIcon, host.DirectoryFollow, host.PasswordSource,
		host.SSHTwoFactorEnabled, host.SSHOTPEntryID, host.SSHOTPAutoFill,
		normalizeShowHiddenFiles(host.SFTPShowHiddenFiles), host.SFTPDefaultDirectory,
		normalizeBackspaceMode(host.BackspaceMode), normalizeEncoding(host.Encoding),
		normalizeConnectTimeout(host.ConnectTimeoutSeconds), host.PostLoginCommand,
		normalizeEnvironmentVariablesJSON(host.EnvironmentVariables), host.X11Forwarding, host.X11Trusted, normalizeRdpDisplayMode(host.RdpDisplayMode), normalizeRdpWidth(host.RdpWidth), normalizeRdpHeight(host.RdpHeight), normalizePostLoginDelay(host.PostLoginCommandDelaySeconds),
		normalizeRdpClipboardMode(host.RdpClipboardMode), normalizeRdpClipboardContent(host.RdpClipboardContent), normalizeHostTagsJSON(host.Tags),
		normalizeVncViewOnly(host.VncViewOnly), normalizeVncShared(host.VncShared), normalizeVncPixelDepth(host.VncPixelDepth),
		normalizeVncScale(host.VncScale), normalizeVncAutoReconnect(host.VncAutoReconnect), normalizeVncAutoReconnectAttempts(host.VncAutoReconnectAttempts),
		normalizeVncTransport(host.VncTransport), normalizeVncWsPath(host.VncWsPath), normalizeVncAllowInsecureTLS(host.VncAllowInsecureTLS),
		normalizeVncRfbPasswordMode(host.VncRfbPasswordMode),
		normalizeConnectionType(host.ConnectionType), normalizeTerminalType(host.ConnectionType, host.TerminalType),

		nullableString(textValue(host.JumpHostID)),
		proxyMode, proxyHost, proxyPort, proxyUsername,
		nullableString(host.ProxyID),
		host.ID,
	)
	if err != nil {
		return model.Host{}, fmt.Errorf("update host: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return model.Host{}, fmt.Errorf("read updated host count: %w", err)
	}
	if count == 0 {
		return model.Host{}, ErrNotFound
	}

	var createdAt string
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM hosts WHERE id = ?`, host.ID).Scan(&createdAt); err != nil {
		return model.Host{}, fmt.Errorf("read updated host: %w", err)
	}
	resultHost.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return model.Host{}, fmt.Errorf("parse updated host creation time: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Host{}, fmt.Errorf("commit updated host: %w", err)
	}
	return resultHost, nil
}

func (s *Store) CountHostsByCredentialID(ctx context.Context, credentialID string) (int, error) {
	if s.codec != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT id, encrypted_payload FROM hosts WHERE encrypted_payload IS NOT NULL`)
		if err != nil {
			return 0, fmt.Errorf("query encrypted hosts by credential ID: %w", err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id string
			var payload []byte
			if err := rows.Scan(&id, &payload); err != nil {
				return 0, fmt.Errorf("scan encrypted host by credential ID: %w", err)
			}
			var host model.Host
			if err := s.openHostPayload(id, payload, &host); err != nil {
				return 0, err
			}
			if host.CredentialID == credentialID {
				count++
			}
		}
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("iterate encrypted hosts by credential ID: %w", err)
		}
		return count, nil
	}
	var count int
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM hosts WHERE credential_id = ?`,
		credentialID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count hosts by credential ID: %w", err)
	}
	return count, nil
}

func (s *Store) CountHostsByJumpHostID(ctx context.Context, jumpHostID string) (int, error) {
	jumpHostID = strings.TrimSpace(jumpHostID)
	if jumpHostID == "" {
		return 0, nil
	}
	var count int
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM hosts WHERE jump_host_id = ?`,
		jumpHostID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count hosts by jump host: %w", err)
	}
	return count, nil
}

var ErrJumpHostReferenced = errors.New("host is referenced as jump host")

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == 787 {
		return true
	}
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

func (s *Store) DeleteHost(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete host transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {

		if isForeignKeyViolation(err) {
			return ErrJumpHostReferenced
		}
		return fmt.Errorf("delete host: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted host count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM command_history WHERE host_id = ?`, id); err != nil {
		return fmt.Errorf("delete host command history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete host: %w", err)
	}
	return nil
}

func normalizeRdpDisplayMode(value string) string {
	if value == "fixed" {
		return "fixed"
	}
	return "fit"
}

func normalizeRdpWidth(value int) int {
	if value < 640 || value > 7680 {
		return 1280
	}
	return value
}

func normalizeRdpHeight(value int) int {
	if value < 480 || value > 4320 {
		return 800
	}
	return value
}

func normalizeRdpClipboardMode(value string) string {
	switch value {
	case "off", "local-to-remote", "remote-to-local":
		return value
	default:
		return "both"
	}
}

func normalizeRdpClipboardContent(value string) string {
	if value == "text-and-files" {
		return "text-and-files"
	}
	return "text"
}

func normalizeVncViewOnly(value bool) bool { return value }

func normalizeVncShared(value bool) bool { return value }

func normalizeVncPixelDepth(value int) int {
	if value == 16 {
		return 16
	}
	return 32
}

func normalizeVncScale(value string) string {
	if value == model.VncScaleActual {
		return model.VncScaleActual
	}
	return model.VncScaleFit
}

func normalizeVncAutoReconnect(value bool) bool { return value }

func normalizeVncAutoReconnectAttempts(value int) int {
	if value >= 0 && value <= model.MaxVncAutoReconnectAttempts {
		return value
	}
	return model.DefaultVncAutoReconnectAttempts
}

func normalizeVncTransport(value string) string {
	if value == model.VncTransportWSS {
		return model.VncTransportWSS
	}
	return model.VncTransportTCP
}

func normalizeVncWsPath(value string) string {
	normalized, err := model.NormalizeVncWsPath(value)
	if err != nil {
		return model.DefaultVncWsPath
	}
	return normalized
}

func normalizeVncAllowInsecureTLS(value bool) bool { return value }

func normalizeVncRfbPasswordMode(value string) string {
	if value == model.VncRfbPasswordSame {
		return model.VncRfbPasswordSame
	}
	return model.VncRfbPasswordAuto
}

func normalizeHostTagsJSON(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func decodeHostTags(raw string, host *model.Host) error {
	if strings.TrimSpace(raw) == "" {
		host.Tags = nil
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return fmt.Errorf("decode host tags: %w", err)
	}
	host.Tags = tags
	return nil
}

func textValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func optionalTextPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalIntPointer(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func applyHostProxy(jumpHostID sql.NullString, mode, host string, port int, username string, proxyID sql.NullString, target *model.Host) {
	target.JumpHostID = optionalTextPointer(strings.TrimSpace(jumpHostID.String))

	target.ProxyID = strings.TrimSpace(proxyID.String)
	mode, host, port, username = normalizeHostProxyColumns(mode, host, port, username)
	target.ProxyMode = &mode
	target.ProxyHost = optionalTextPointer(host)
	target.ProxyPort = optionalIntPointer(port)
	target.ProxyUsername = optionalTextPointer(username)
}

func normalizeProxyMode(value string) string {
	switch strings.TrimSpace(value) {
	case model.ProxyModeSOCKS5:
		return model.ProxyModeSOCKS5
	case model.ProxyModeHTTP:
		return model.ProxyModeHTTP
	default:
		return model.ProxyModeNone
	}
}

func normalizeHostProxyColumns(mode, host string, port int, username string) (string, string, int, string) {
	mode = normalizeProxyMode(mode)
	if mode == model.ProxyModeNone {
		return mode, "", 0, ""
	}
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	if port < 1 || port > 65535 {

		port = 0
	}
	return mode, host, port, username
}

func normalizeSSHOTPSettings(host *model.Host) {
	if host == nil {
		return
	}
	host.SSHOTPEntryID = strings.TrimSpace(host.SSHOTPEntryID)
	if host.Protocol != model.ProtocolSSH || !host.SSHTwoFactorEnabled {
		host.SSHTwoFactorEnabled = false
		host.SSHOTPEntryID = ""
		host.SSHOTPAutoFill = false
		return
	}
	if host.SSHOTPEntryID == "" {
		host.SSHOTPAutoFill = false
	}
}
