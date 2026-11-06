package host

import (
	"context"
	"errors"
	"strings"
	"sync"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

func (s *HostService) lockCredentialLink(
	ctx context.Context,
	credentialID string,
) (func(), error) {
	links, lookup := s.credentialLinkSnapshot()
	if links == nil {
		if credentialID == "" {
			return func() {}, nil
		}
		return nil, errno.New(errno.CodeSessionClosed, "host.credential_not_ready", "")
	}
	links.RLock()
	if credentialID == "" {
		return links.RUnlock, nil
	}
	if lookup == nil {
		links.RUnlock()
		return nil, errno.New(errno.CodeSessionClosed, "host.credential_not_ready", "")
	}
	exists, err := lookup(ctx, credentialID)
	if err != nil {
		links.RUnlock()
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return nil, errno.New(errno.CodePermissionDenied, "host.credential_locked", "")
		}
		return nil, errno.New(errno.CodeStoreFailed, "host.credential_check_failed", "")
	}
	if !exists {
		links.RUnlock()
		return nil, errno.New(errno.CodeNotFound, "credential.not_found", "")
	}
	return links.RUnlock, nil
}

func (s *HostService) lockCredentialLinkForUpdate(
	ctx context.Context,
	hostID string,
	credentialID string,
) (func(), error) {
	links, lookup := s.credentialLinkSnapshot()
	if links == nil {
		if credentialID != "" {
			return nil, errno.New(errno.CodeSessionClosed, "host.credential_not_ready", "")
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		if s.store == nil {
			return nil, ErrHostServiceNotReady
		}
		if _, err := s.store.GetHost(ctx, hostID); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, errno.New(errno.CodeNotFound, "host.not_found", "")
			}
			return nil, errno.New(errno.CodeStoreFailed, "host.read_failed", "")
		}
		return func() {}, nil
	}
	links.RLock()

	s.mu.RLock()
	if s.store == nil {
		s.mu.RUnlock()
		links.RUnlock()
		return nil, ErrHostServiceNotReady
	}
	existing, err := s.store.GetHost(ctx, hostID)
	s.mu.RUnlock()
	if errors.Is(err, storage.ErrNotFound) {
		links.RUnlock()
		return nil, errno.New(errno.CodeNotFound, "host.not_found", "")
	}
	if err != nil {
		links.RUnlock()
		return nil, errno.New(errno.CodeStoreFailed, "host.read_failed", "")
	}
	if credentialID == existing.CredentialID {
		return links.RUnlock, nil
	}
	if credentialID != "" && lookup == nil {
		links.RUnlock()
		return nil, errno.New(errno.CodeSessionClosed, "host.credential_not_ready", "")
	}
	if credentialID != "" {
		exists, lookupErr := lookup(ctx, credentialID)
		if lookupErr != nil {
			links.RUnlock()
			if errors.Is(lookupErr, model.ErrCredentialVaultLocked) {
				return nil, errno.New(errno.CodePermissionDenied, "host.credential_locked", "")
			}
			return nil, errno.New(errno.CodeStoreFailed, "host.credential_check_failed", "")
		}
		if !exists {
			links.RUnlock()
			return nil, errno.New(errno.CodeNotFound, "credential.not_found", "")
		}
	}
	return links.RUnlock, nil
}

func (s *HostService) credentialLinkSnapshot() (*sync.RWMutex, func(context.Context, string) (bool, error)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.credentialLinks, s.credentials
}

func (s *HostService) credentialReferenceCount(ctx context.Context, id string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return 0, ErrHostServiceNotReady
	}
	return s.store.CountHostsByCredentialID(ctx, id)
}

func (s *HostService) sessionHost(ctx context.Context, id string) (model.Host, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Host{}, model.ErrInvalidSession
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Host{}, ErrHostServiceNotReady
	}
	host, err := s.store.GetHost(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.Host{}, errno.New(errno.CodeNotFound, "host.not_found", "")
		}
		return model.Host{}, errno.New(errno.CodeStoreFailed, "host.session_read_failed", "")
	}
	return host, nil
}

func (s *HostService) UseHostProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || use == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.UseHostProxyPassword(ctx, id, use)
}
