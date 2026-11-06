package session

import (
	"context"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type DetectIconInput struct {
	HostID       string               `json:"hostId,omitempty"`
	Protocol     string               `json:"protocol,omitempty"`
	Address      string               `json:"address,omitempty"`
	Port         int                  `json:"port,omitempty"`
	Username     string               `json:"username,omitempty"`
	CredentialID string               `json:"credentialId,omitempty"`
	Kind         model.CredentialKind `json:"kind,omitempty"`
	Secret       string               `json:"secret,omitempty"`
}

func (s *Service) DetectIcon(ctx context.Context, input DetectIconInput) (string, error) {
	if _, _, err := s.ready(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hostID := strings.TrimSpace(input.HostID)

	var host model.Host
	if hostID != "" {
		loaded, err := s.hosts.SessionHost(ctx, hostID)
		if err != nil {
			return "", testConnectionHostError(err)
		}
		host = loaded
	}
	target, err := detectIconTarget(input, host)
	if err != nil {
		return "", err
	}

	if hostID != "" {
		if sessionID, ok := s.activeSessionForHost(ctx, hostID); ok {
			detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			icon, detectErr := s.manager.DetectRemoteIcon(detectCtx, sessionID)
			cancel()
			if detectErr == nil && icon != "" {
				return icon, nil
			}

		}
	}

	return s.detectIconWithoutSession(ctx, input, host, target)
}

func detectIconTarget(input DetectIconInput, host model.Host) (sshclient.Target, error) {
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = host.Protocol
	}
	if protocol == "" {
		protocol = "ssh"
	}
	if protocol != model.ProtocolSSH {

		return sshclient.Target{}, errno.New(errno.CodeInvalidArg, "session.icon_detect_unsupported", "")
	}
	target := sshclient.Target{
		Address:  strings.TrimSpace(input.Address),
		Port:     input.Port,
		Username: strings.TrimSpace(input.Username),
	}
	if target.Address == "" {
		target.Address = strings.TrimSpace(host.Address)
	}
	if target.Username == "" {
		target.Username = strings.TrimSpace(host.Username)
	}
	if target.Port == 0 {
		target.Port = host.Port
	}
	if target.Address == "" || target.Username == "" || target.Port < 1 || target.Port > 65535 {
		return sshclient.Target{}, errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}
	return target, nil
}

func (s *Service) activeSessionForHost(ctx context.Context, hostID string) (string, bool) {
	sessions, err := s.manager.List(ctx)
	if err != nil {
		return "", false
	}
	for _, item := range sessions {
		if item.HostID != hostID {
			continue
		}
		switch item.State {
		case model.SessionStateConnected, model.SessionStateAuthenticating:
			return item.ID, true
		}
	}
	return "", false
}

func (s *Service) detectIconWithoutSession(
	ctx context.Context,
	input DetectIconInput,
	host model.Host,
	target sshclient.Target,
) (string, error) {
	detectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	credentialID := strings.TrimSpace(input.CredentialID)
	if credentialID == "" {
		credentialID = strings.TrimSpace(host.CredentialID)
	}
	switch {
	case credentialID != "":
		icon, err := s.connector.DetectStoredRemoteIconForHost(detectCtx, host, credentialID, target)
		return icon, testConnectionError(err)
	case input.Kind == model.CredentialKindPassword && input.Secret != "":
		secret := []byte(input.Secret)
		defer clear(secret)
		icon, err := s.connector.DetectRemoteIconForHost(detectCtx, host, target, model.CredentialKindPassword, secret)
		return icon, testConnectionError(err)
	case host.ID != "" && host.PasswordConfigured:
		var icon string
		err := s.hosts.UseHostPassword(detectCtx, host.ID, func(secret []byte) error {
			detected, detectErr := s.connector.DetectRemoteIconForHost(detectCtx, host, target, model.CredentialKindPassword, secret)
			if detectErr != nil {
				return detectErr
			}
			icon = detected
			return nil
		})
		if err != nil {
			return "", testConnectionError(err)
		}
		return icon, nil
	default:

		return "", errno.New(errno.CodeInvalidArg, "session.credential_required", "")
	}
}
