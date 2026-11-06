package session

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
	"context"
	"errors"
	"strings"
)

func (s *Service) OpenSession(ctx context.Context, hostID string) (OpenSessionResult, error) {
	manager, _, err := s.ready()
	if err != nil {
		return OpenSessionResult{}, err
	}
	connector, err := s.connectorFor(ctx, hostID)
	if err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	session, err := manager.OpenWith(ctx, hostID, connector)
	if err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	token, err := s.issueStreamToken(manager, session.ID)
	if err != nil {
		closeSessionAfterToken(manager, session.ID)
		return OpenSessionResult{}, errno.New(errno.CodeUnknown, "session.token_failed", "")
	}
	return OpenSessionResult{Session: session, StreamToken: token}, nil
}

func (s *Service) connectorFor(ctx context.Context, hostID string) (SessionConnector, error) {
	_, connector, err := s.ready()
	if err != nil {
		return nil, err
	}
	host, err := s.hosts.SessionHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	switch host.Protocol {
	case model.ProtocolTelnet:
		s.mu.RLock()
		telnetConnector := s.telnetConnector
		s.mu.RUnlock()
		if telnetConnector == nil {
			return nil, ErrServiceNotReady
		}
		return telnetConnector, nil
	case model.ProtocolSerial:
		s.mu.RLock()
		serialConnector := s.serialConnector
		s.mu.RUnlock()
		if serialConnector == nil {
			return nil, ErrServiceNotReady
		}
		return serialConnector, nil
	case model.ProtocolRDP:

		return nil, errno.New(errno.CodeInvalidArg, "session.protocol_desktop", "")
	default:
		return connector, nil
	}
}

func (s *Service) requireSSHSession(ctx context.Context, id string) error {
	manager, _, err := s.ready()
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot, err := manager.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return sessionServiceError(err)
	}
	if snapshot.Kind != model.SessionKindSSH {
		return errno.New(errno.CodeInvalidArg, "session.not_ssh", "")
	}
	return nil
}

func (s *Service) OpenSftpConnection(ctx context.Context, hostID string) (model.Session, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.Session{}, err
	}
	session, err := manager.OpenForSftp(ctx, hostID)
	if err != nil {
		return model.Session{}, sessionServiceError(err)
	}
	return session, nil
}

func (s *Service) OpenLocalSession(
	ctx context.Context,
	shellPath string,
	workingDirectory string,
) (OpenSessionResult, error) {
	manager, _, err := s.ready()
	if err != nil {
		return OpenSessionResult{}, err
	}
	plan, err := sshclient.ResolveLocalShellPlan(sshclient.LocalShellOptions{
		Path:             shellPath,
		WorkingDirectory: workingDirectory,
	})
	if err != nil {

		if errors.Is(err, sshclient.ErrLocalShellUnavailable) {
			return OpenSessionResult{}, errno.New(errno.CodeInvalidArg, "session.local_shell_unavailable", "")
		}
		return OpenSessionResult{}, sessionServiceError(err)
	}
	session, err := manager.OpenLocal(ctx, plan)
	if err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	token, err := s.issueStreamToken(manager, session.ID)
	if err != nil {
		closeSessionAfterToken(manager, session.ID)
		return OpenSessionResult{}, errno.New(errno.CodeUnknown, "session.token_failed", "")
	}
	return OpenSessionResult{Session: session, StreamToken: token, Warnings: plan.Warnings}, nil
}

func (s *Service) PrepareSessionHandover(ctx context.Context, id string) error {
	manager, _, err := s.ready()
	if err != nil {
		return err
	}
	return sessionServiceError(manager.PrepareSessionHandover(ctx, id))
}

func (s *Service) RebindSession(ctx context.Context, id string) (OpenSessionResult, error) {
	manager, _, err := s.ready()
	if err != nil {
		return OpenSessionResult{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return OpenSessionResult{}, errno.New(errno.CodeInvalidArg, "session.id_required", "")
	}

	if err := manager.PrepareSessionHandover(ctx, id); err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	snapshot, err := manager.Get(ctx, id)
	if err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	if snapshot.State == model.SessionStateClosed || snapshot.State == model.SessionStateFailed {
		return OpenSessionResult{}, errno.New(errno.CodeSessionClosed, "session.closed", "")
	}
	token, err := s.issueStreamToken(manager, id)
	if err != nil {
		return OpenSessionResult{}, errno.New(errno.CodeUnknown, "session.token_failed", "")
	}
	return OpenSessionResult{Session: snapshot, StreamToken: token}, nil
}

func (s *Service) GetSession(ctx context.Context, id string) (model.Session, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.Session{}, err
	}
	session, err := manager.Get(ctx, id)
	if err != nil {
		return model.Session{}, sessionServiceError(err)
	}
	return session, nil
}

func (s *Service) ReadResources(ctx context.Context, id string) (model.ResourceSnapshot, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.ResourceSnapshot{}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, resourceReadTimeout)
	defer cancel()
	return manager.ReadResources(probeCtx, id)
}

func (s *Service) ActiveSessionCount() int {
	if s == nil {
		return 0
	}
	manager, err := s.Manager()
	if err != nil {
		return 0
	}
	return manager.ActiveCount()
}

func (s *Service) ListSessions(ctx context.Context) ([]model.Session, error) {
	manager, _, err := s.ready()
	if err != nil {
		return nil, err
	}
	sessions, err := manager.List(ctx)
	if err != nil {
		return nil, sessionServiceError(err)
	}
	result := make([]model.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.Purpose == model.SessionPurposeSftp {
			continue
		}
		result = append(result, session)
	}
	return result, nil
}

func (s *Service) CloseSession(ctx context.Context, id string) error {
	manager, connector, err := s.ready()
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)

	var recordingErr error

	for _, mode := range []string{recordingModeSegment, recordingModeFull} {
		if _, exists := s.recordings.peek(id, mode); !exists {
			continue
		}

		if _, err := s.recordings.finishRecording(id, mode, true); err != nil && recordingErr == nil {
			recordingErr = err
		}
	}

	s.recordings.flushPending()
	s.tokens.Revoke(id)
	if err := manager.Close(ctx, id); err != nil {
		return sessionServiceError(err)
	}
	connector.Forget(id)
	s.mu.RLock()
	telnetConnector := s.telnetConnector
	s.mu.RUnlock()
	if telnetConnector != nil {
		telnetConnector.Forget(id)
	}
	if recordingErr != nil {
		return errno.Wrap(errno.CodeStoreFailed, "recording.close_failed", recordingErr)
	}
	return nil
}

func (s *Service) SessionHandoverPending(ctx context.Context, id string) bool {
	if s == nil {
		return false
	}
	manager, _, err := s.ready()
	if err != nil {
		return false
	}
	return manager.HandoverPending(ctx, id)
}

func (s *Service) CloseSessionFromWindow(ctx context.Context, id string) error {
	if s == nil {
		return ErrServiceNotReady
	}
	if s.SessionHandoverPending(ctx, id) {
		return nil
	}
	return s.CloseSession(ctx, id)
}

func (s *Service) GetSessionHostKeyPending(id string) (model.HostKeyPending, error) {
	_, connector, err := s.ready()
	if err != nil {
		return model.HostKeyPending{}, err
	}
	if err := s.requireSSHSession(context.Background(), id); err != nil {
		return model.HostKeyPending{}, err
	}
	id = strings.TrimSpace(id)
	pending, ok := connector.PendingHostKey(id)
	if !ok {
		return model.HostKeyPending{}, errno.New(errno.CodeNotFound, "session.no_challenge", "")
	}
	return pending, nil
}

func (s *Service) GetSessionHostKeyChallenge(id string) (model.HostKeyChallenge, error) {
	pending, err := s.GetSessionHostKeyPending(id)
	if err != nil {
		return model.HostKeyChallenge{}, err
	}
	return pending.Challenge, nil
}

func (s *Service) GetSessionAuthenticationChallenge(id string) (AuthenticationChallenge, error) {
	_, connector, err := s.ready()
	if err != nil {
		return AuthenticationChallenge{}, err
	}
	if err := s.requireSSHSession(context.Background(), id); err != nil {
		return AuthenticationChallenge{}, err
	}
	challenge, ok := connector.AuthenticationChallenge(strings.TrimSpace(id))
	if !ok {
		return AuthenticationChallenge{}, errno.New(errno.CodeNotFound, "session.no_authentication_challenge", "")
	}
	return challenge, nil
}

func (s *Service) SubmitSessionAuthentication(id string, input AuthenticationInput) error {
	_, connector, err := s.ready()
	if err != nil {
		return err
	}
	if err := s.requireSSHSession(context.Background(), id); err != nil {
		return err
	}
	return connector.SubmitAuthentication(strings.TrimSpace(id), input)
}

func (s *Service) GetSessionFailure(id string) (string, error) {
	_, connector, err := s.ready()
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	message, ok := connector.Failure(id)
	if !ok {

		s.mu.RLock()
		telnetConnector := s.telnetConnector
		s.mu.RUnlock()
		if telnetConnector != nil {
			message, ok = telnetConnector.Failure(id)
		}
	}
	if !ok {
		return "", errno.New(errno.CodeNotFound, "session.no_failure", "")
	}
	return message, nil
}

func (s *Service) TrustSessionHostKey(
	ctx context.Context,
	id string,
) (OpenSessionResult, error) {
	manager, connector, err := s.ready()
	if err != nil {
		return OpenSessionResult{}, err
	}
	if err := s.requireSSHSession(ctx, id); err != nil {
		return OpenSessionResult{}, err
	}
	id = strings.TrimSpace(id)
	if err := connector.TrustUnknown(id); err != nil {
		return OpenSessionResult{}, errno.New(errno.CodeStoreFailed, "session.trust_failed", "")
	}
	session, err := manager.Retry(ctx, id)
	if err != nil {
		return OpenSessionResult{}, sessionServiceError(err)
	}
	token, err := s.issueStreamToken(manager, session.ID)
	if err != nil {
		closeSessionAfterToken(manager, session.ID)
		return OpenSessionResult{}, errno.New(errno.CodeUnknown, "session.token_failed", "")
	}
	return OpenSessionResult{Session: session, StreamToken: token}, nil
}
