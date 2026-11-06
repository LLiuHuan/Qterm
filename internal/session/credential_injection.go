package session

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/vault"
)

var errEmptyCredentialSecret = errors.New("credential secret is empty")

func (s *Service) InjectCredentialIntoSession(ctx context.Context, sessionID string, credentialID string) error {
	manager, _, err := s.ready()
	if err != nil {
		return credentialInjectionError(err)
	}
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "session.invalid", "")
	}
	sessionID = strings.TrimSpace(sessionID)
	credentialID = strings.TrimSpace(credentialID)
	if sessionID == "" {
		return errno.New(errno.CodeInvalidArg, "session.invalid", "")
	}
	if credentialID == "" {
		return errno.New(errno.CodeInvalidArg, "session.credential_required", "")
	}
	if err := manager.RequireTerminal(ctx, sessionID); err != nil {
		return credentialInjectionError(err)
	}
	s.mu.RLock()
	credentials := s.credentials
	s.mu.RUnlock()
	if credentials == nil {
		return ErrServiceNotReady
	}
	err = credentials.UseCredentialSecret(ctx, credentialID, func(_ model.CredentialKind, secret []byte) error {
		if len(secret) == 0 {
			return errEmptyCredentialSecret
		}

		payload := make([]byte, 0, len(secret)+1)
		payload = append(payload, secret...)
		payload = append(payload, '\r')
		defer clear(payload)
		return manager.WriteSensitiveInput(ctx, sessionID, payload)
	})
	return credentialInjectionError(err)
}

func credentialInjectionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errno.Extract(err) != nil:
		return err
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, model.ErrCredentialVaultLocked):
		return errno.New(errno.CodePermissionDenied, "credential.locked", "")
	case errors.Is(err, vault.ErrNotFound):
		return errno.New(errno.CodeNotFound, "credential.not_found", "")
	case errors.Is(err, model.ErrInvalidCredential):
		return errno.New(errno.CodeInvalidArg, "credential.invalid", "")
	case errors.Is(err, errEmptyCredentialSecret):
		return errno.New(errno.CodeInvalidArg, "credential.secret_invalid", "")
	case errors.Is(err, model.ErrInvalidSession):
		return errno.New(errno.CodeInvalidArg, "session.invalid", "")
	case errors.Is(err, ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, "session.not_found", "")
	case errors.Is(err, ErrSessionNotConnected), errors.Is(err, ErrSessionTerminalDetached):
		return errno.New(errno.CodeSessionClosed, "session.not_connected", "")
	case errors.Is(err, ErrSessionTerminalUnsupported):
		return errno.New(errno.CodeInvalidArg, "session.terminal_unsupported", "")
	case errors.Is(err, ErrServiceNotReady),
		errors.Is(err, ErrSessionManagerNotReady),
		errors.Is(err, ErrSessionManagerClosed):
		return errno.New(errno.CodeSessionClosed, "session.not_ready", "")
	default:
		return errno.New(errno.CodeUnknown, "session.credential_inject_failed", "")
	}
}
