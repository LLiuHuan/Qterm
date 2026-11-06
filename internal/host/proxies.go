package host

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	storage "changeme/internal/store"
)

func (s *HostService) WithStore(fn func(*storage.Store) error) error {
	if s == nil || fn == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return fn(s.store)
}

func (s *HostService) requireProxyExists(ctx context.Context, proxyID string) error {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if _, err := s.store.ProxyByID(ctx, proxyID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "proxy.not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "proxy.read_failed", "")
	}
	return nil
}
