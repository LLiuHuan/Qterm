package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"changeme/internal/assetdata"
)

func (s *Store) ReplaceAssets(ctx context.Context, data assetdata.Snapshot) error {
	if s == nil || s.db == nil || s.codec == nil {
		return errors.New("store is not ready")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin asset replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return fmt.Errorf("defer asset constraints: %w", err)
	}
	for _, statement := range []string{
		`DELETE FROM snippets`,
		`DELETE FROM snippet_groups`,

		`DELETE FROM credential_prompt_rules`,
		`DELETE FROM hosts`,

		`DELETE FROM proxies`,
		`UPDATE groups SET parent_id = NULL`,
		`DELETE FROM groups`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("clear asset records: %w", err)
		}
	}

	for _, proxy := range data.Proxies {
		payload, err := s.sealProxyPayload(proxy.ID, proxy.Password)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO proxies (id, name, mode, host, port, username, created_at, updated_at, encrypted_payload)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			proxy.ID, proxy.Name, proxy.Mode, proxy.Host, proxy.Port, proxy.Username,
			proxy.CreatedAt, proxy.UpdatedAt, payload); err != nil {
			clear(payload)
			return fmt.Errorf("insert asset proxy: %w", err)
		}
		clear(payload)
	}
	for _, group := range data.Groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO groups(id,name,sort_order,created_at,updated_at,parent_id) VALUES(?,?,?,?,?,NULL)`, group.ID, group.Name, group.SortOrder, formatTime(group.CreatedAt), formatTime(group.UpdatedAt)); err != nil {
			return fmt.Errorf("insert asset group: %w", err)
		}
	}
	for _, group := range data.Groups {
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET parent_id = NULLIF(?, '') WHERE id = ?`, group.ParentID, group.ID); err != nil {
			return fmt.Errorf("restore asset group hierarchy: %w", err)
		}
	}
	for _, host := range data.Hosts {
		passwordSource := host.PasswordSource
		if passwordSource == "" {
			switch {
			case host.CredentialID != "":
				passwordSource = "none"
			case len(host.Password) > 0:
				passwordSource = "saved"
			default:
				passwordSource = "none"
			}
		}
		directoryFollow := host.DirectoryFollow
		if directoryFollow == "" {
			directoryFollow = "inherit"
		}
		payload, err := sealAssetHost(s.codec, host)
		if err != nil {
			return err
		}

		assetProxyMode, assetProxyHost, assetProxyPort, assetProxyUsername := normalizeHostProxyColumns(
			textValue(host.ProxyMode), textValue(host.ProxyHost), intValue(host.ProxyPort),
			textValue(host.ProxyUsername),
		)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO hosts (
				id, icon, color, name, protocol, address, port, username, group_id, sort_order,
				credential_id, notes, created_at, updated_at, encrypted_payload,
				ssh_keep_alive, latency_detection, auto_detect_os_icon, directory_follow, password_source,
				sftp_show_hidden_files, sftp_default_directory,
				backspace_mode, encoding, connect_timeout_seconds, post_login_command, environment_variables,
				x11_forwarding, x11_trusted, rdp_display_mode, rdp_width, rdp_height, post_login_command_delay_seconds,
				rdp_clipboard_mode, rdp_clipboard_content, tags,
				vnc_view_only, vnc_shared, vnc_pixel_depth,
				vnc_scale, vnc_auto_reconnect, vnc_auto_reconnect_attempts,
				vnc_transport, vnc_ws_path, vnc_allow_insecure_tls, vnc_rfb_password_mode,
				connection_type, terminal_type,
				proxy_mode, proxy_host, proxy_port, proxy_username,
				jump_host_id, proxy_id
			) VALUES (
				?, '', ?, '', 'ssh', '', 22, '', NULLIF(?, ''), ?,
				'', '', ?, ?, ?,
				?, ?, ?, ?, ?,
				?, ?,
				?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?,
				?, ?, ?,
				?, ?, ?,
				?, ?, ?,
				?, ?, ?, ?,
				?, ?,
				?, ?, ?, ?,
				?, ?
			)`,
			host.ID, host.Color, host.GroupID, host.SortOrder, formatTime(host.CreatedAt), formatTime(host.UpdatedAt), payload,
			host.SSHKeepAlive, host.LatencyDetection, host.AutoDetectOSIcon, directoryFollow, passwordSource,

			normalizeShowHiddenFiles(host.SFTPShowHiddenFiles), host.SFTPDefaultDirectory,
			normalizeBackspaceMode(host.BackspaceMode), normalizeEncoding(host.Encoding),
			normalizeConnectTimeout(host.ConnectTimeoutSeconds), host.PostLoginCommand,
			normalizeEnvironmentVariablesJSON(host.EnvironmentVariables),
			host.X11Forwarding, host.X11Trusted,
			normalizeRdpDisplayMode(host.RdpDisplayMode), normalizeRdpWidth(host.RdpWidth), normalizeRdpHeight(host.RdpHeight),
			normalizePostLoginDelay(host.PostLoginCommandDelaySeconds),
			normalizeRdpClipboardMode(host.RdpClipboardMode), normalizeRdpClipboardContent(host.RdpClipboardContent), normalizeHostTagsJSON(host.Tags),
			normalizeVncViewOnly(host.VncViewOnly), normalizeVncShared(host.VncShared),
			normalizeVncPixelDepth(host.VncPixelDepth), normalizeVncScale(host.VncScale),
			normalizeVncAutoReconnect(host.VncAutoReconnect),
			normalizeVncAutoReconnectAttempts(host.VncAutoReconnectAttempts),

			normalizeVncTransport(host.VncTransport), normalizeVncWsPath(host.VncWsPath),
			normalizeVncAllowInsecureTLS(host.VncAllowInsecureTLS),

			normalizeVncRfbPasswordMode(host.VncRfbPasswordMode),

			normalizeConnectionType(host.ConnectionType), normalizeTerminalType(host.ConnectionType, host.TerminalType),
			assetProxyMode, assetProxyHost, assetProxyPort, assetProxyUsername,

			nullableString(textValue(host.JumpHostID)), nullableString(host.ProxyID))
		clear(payload)
		if err != nil {
			return fmt.Errorf("insert asset host: %w", err)
		}
	}
	for _, group := range data.SnippetGroups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO snippet_groups(id,name,created_at,updated_at) VALUES(?,?,?,?)`, group.ID, group.Name, formatTime(group.CreatedAt), formatTime(group.UpdatedAt)); err != nil {
			return fmt.Errorf("insert command group: %w", err)
		}
	}
	for _, snippet := range data.Snippets {
		hostIDs, err := json.Marshal(snippet.HostIDs)
		if err != nil {
			return fmt.Errorf("encode command hosts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO snippets (
				id, name, content, group_id, auto_execute, created_at, updated_at,
				host_ids, snippet_group_id, asset_group_id, sort_order, icon, color, pinned
			) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)`,
			snippet.ID, snippet.Name, snippet.Content, snippet.AutoExecute, formatTime(snippet.CreatedAt), formatTime(snippet.UpdatedAt), string(hostIDs), snippet.GroupID, snippet.AssetGroupID, snippet.SortOrder, snippet.Icon, snippet.Color, snippet.Pinned); err != nil {
			return fmt.Errorf("insert command: %w", err)
		}
	}

	for _, rule := range data.CredentialPromptRules {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credential_prompt_rules (id, kind, pattern, enabled, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rule.ID, rule.Kind, rule.Pattern, boolInt(rule.Enabled), rule.SortOrder,
			formatTime(rule.CreatedAt), formatTime(rule.UpdatedAt)); err != nil {
			return fmt.Errorf("insert credential prompt rule: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit asset replacement: %w", err)
	}
	return nil
}

func sealAssetHost(codec EncryptedPayloadCodec, host assetdata.Host) ([]byte, error) {
	passwordSource := host.PasswordSource
	if passwordSource == "" {
		if host.CredentialID != "" || len(host.Password) == 0 {
			passwordSource = "none"
		} else {
			passwordSource = "saved"
		}
	}

	serialBaud, serialDataBits := host.SerialBaud, host.SerialDataBits
	if serialBaud <= 0 {
		serialBaud = 115200
	}
	if serialDataBits <= 0 {
		serialDataBits = 8
	}
	serialParity := host.SerialParity
	if serialParity == "" {
		serialParity = "none"
	}
	serialStopBits := host.SerialStopBits
	if serialStopBits == "" {
		serialStopBits = "1"
	}
	payload := encryptedHostPayload{
		Icon: host.Icon, Name: host.Name, Protocol: host.Protocol, Address: host.Address,
		Port: host.Port, Username: host.Username, CredentialID: host.CredentialID,
		PasswordSource: passwordSource,
		Notes:          host.Notes, SSHKeepAlive: boolPtr(host.SSHKeepAlive), LatencyDetection: boolPtr(host.LatencyDetection),
		AutoDetectOSIcon: boolPtr(host.AutoDetectOSIcon), DirectoryFollow: host.DirectoryFollow, Password: string(host.Password),
		ProxyPassword:  string(host.ProxyPassword),
		SerialBaud:     serialBaud,
		SerialDataBits: serialDataBits,
		SerialParity:   serialParity,
		SerialStopBits: serialStopBits,
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode asset host: %w", err)
	}
	defer clear(plaintext)
	return codec.Seal("host", host.ID, plaintext)
}
