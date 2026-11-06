package app

import (
	"context"
	"strings"

	"changeme/internal/credential"
	"changeme/internal/errno"
	"changeme/internal/host"
	"changeme/internal/model"
	"changeme/internal/remotedesktop"
)

func splitRdpUsername(username string) (domain, user string) {
	username = strings.TrimSpace(username)
	if index := strings.Index(username, `\`); index > 0 && index < len(username)-1 {
		return username[:index], username[index+1:]
	}
	return "", username
}

const rdpDefaultHostIcon = "windows-10"

func (s *Service) applyDefaultRdpHostIcon(ctx context.Context, record model.Host) {
	if s == nil || s.hosts == nil {
		return
	}
	if strings.TrimSpace(record.Icon) != "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = host.SetDetectedHostIcon(s.hosts, ctx, record.ID, rdpDefaultHostIcon)
}

func (s *Service) rdpHostProfile(ctx context.Context, hostID string) (model.RdpHostProfile, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return model.RdpHostProfile{}, rdpError(errno.CodeInvalidArg, "rdp.hostRequired", nil)
	}
	record, err := host.SessionHost(s.hosts, ctx, hostID)
	if err != nil {
		return model.RdpHostProfile{}, err
	}
	if record.Protocol != model.ProtocolRDP {
		return model.RdpHostProfile{}, rdpError(errno.CodeInvalidArg, "rdp.hostNotRdp", nil)
	}
	port := record.Port
	if port <= 0 {
		port = 3389
	}
	domain, username := splitRdpUsername(record.Username)
	return model.RdpHostProfile{
		HostID:            record.ID,
		Name:              record.Name,
		Address:           record.Address,
		Port:              port,
		Username:          username,
		Domain:            domain,
		PasswordAvailable: record.PasswordConfigured || record.CredentialID != "",
		PasswordSource:    record.PasswordSource,

		DisplayMode: normalizedRdpDisplayMode(record.RdpDisplayMode),
		Width:       normalizedRdpDimension(record.RdpWidth, 1280),
		Height:      normalizedRdpDimension(record.RdpHeight, 800),

		ClipboardMode:    remotedesktop.NormalizeClipboardSettings(remotedesktop.ClipboardSettings{Mode: record.RdpClipboardMode}).Mode,
		ClipboardContent: remotedesktop.NormalizeClipboardSettings(remotedesktop.ClipboardSettings{Content: record.RdpClipboardContent}).Content,
	}, nil
}

func (s *Service) RdpHostProfile(ctx context.Context, hostID string) (model.RdpHostProfile, error) {
	if s == nil || s.hosts == nil {
		return model.RdpHostProfile{}, rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	return publicValue(s.rdpHostProfile(ctx, hostID))
}

func (s *Service) resolveRdpPassword(ctx context.Context, record model.Host, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if record.CredentialID != "" {
		var secret string
		err := credential.UseCredentialSecret(s.credentials, ctx, record.CredentialID,
			func(kind model.CredentialKind, value []byte) error {
				if kind != model.CredentialKindPassword {
					return rdpError(errno.CodeInvalidArg, "rdp.credentialNotPassword", nil)
				}
				secret = string(value)
				return nil
			})
		if err != nil {
			return "", err
		}
		return secret, nil
	}
	var secret string
	if err := s.hosts.UseHostPassword(ctx, record.ID, func(value []byte) error {
		secret = string(value)
		return nil
	}); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Service) RdpOpenHost(
	ctx context.Context,
	hostID string,
	width, height int,
	password string,
	engine string,
) (model.RdpSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := s.rdpManager(); err != nil {
		return model.RdpSession{}, publicError(err)
	}
	hostID = strings.TrimSpace(hostID)
	record, err := host.SessionHost(s.hosts, ctx, hostID)
	if err != nil {
		return model.RdpSession{}, publicError(err)
	}
	if record.Protocol != model.ProtocolRDP {
		return model.RdpSession{}, publicError(rdpError(errno.CodeInvalidArg, "rdp.hostNotRdp", nil))
	}
	secret, err := s.resolveRdpPassword(ctx, record, password)
	if err != nil {
		return model.RdpSession{}, publicError(err)
	}
	if secret == "" {
		return model.RdpSession{}, publicError(rdpError(errno.CodeInvalidArg, "rdp.passwordRequired", nil))
	}
	domain, username := splitRdpUsername(record.Username)
	port := record.Port
	if port <= 0 {
		port = 3389
	}

	fingerprint, err := s.trustedRdpFingerprint(record.Address, port)
	if err != nil {
		return model.RdpSession{}, publicError(err)
	}

	options := rdpOptionsFor(record.Address, port, username, domain, secret, width, height, engine, fingerprint,
		remotedesktop.ClipboardSettings{Mode: record.RdpClipboardMode, Content: record.RdpClipboardContent})
	session, err := s.openRdpOptions(ctx, options, &record)
	if err != nil {
		return model.RdpSession{}, publicError(classifyRdpOpenError(err))
	}

	s.applyDefaultRdpHostIcon(ctx, record)
	stats := session.Stats()
	return model.RdpSession{
		ID:            session.ID(),
		Host:          record.Address,
		Engine:        stats.Engine,
		Width:         stats.Width,
		Height:        stats.Height,
		Connected:     stats.Connected,
		SecurityLabel: stats.SecurityLabel,
		Channels:      stats.Channels,
	}, nil
}

func (s *Service) OpenRdpWindow(ctx context.Context, hostID, title string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.hostRequired", nil)
	}
	if s == nil || s.openWindow == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	s.openWindow(ctx, rdpWindowURL(hostID, title), nil)
	return nil
}

func normalizedRdpDisplayMode(value string) string {
	if strings.TrimSpace(value) == "fixed" {
		return "fixed"
	}
	return "fit"
}

func normalizedRdpDimension(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
