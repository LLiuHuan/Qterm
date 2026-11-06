package host

import (
	"changeme/internal/errno"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"changeme/internal/model"
	"changeme/internal/security"
	storage "changeme/internal/store"
)

var (
	ErrHostServiceNotReady = errno.New(errno.CodeSessionClosed, "host.not_ready", "")
)

type HostService struct {
	mu        sync.RWMutex
	store     *storage.Store
	configDir func() (string, error)

	recordingDirOverride string
	credentialLinks      *sync.RWMutex
	credentials          func(context.Context, string) (bool, error)
	security             *security.Manager

	sessionHistoryReconciled bool
}

func AttachCredentialLookup(s *HostService, links any, lookup func(context.Context, string) (bool, error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil || s.credentialLinks != nil || s.credentials != nil {
		return
	}
	s.credentialLinks = rwMutexLink(links)
	s.credentials = lookup
}

func rwMutexLink(value any) *sync.RWMutex {
	links, _ := value.(*sync.RWMutex)
	return links
}

func NewHostService() *HostService {
	return newHostService(os.UserConfigDir)
}

func NewHostServiceWithSecurity(sec *security.Manager) *HostService {
	s := newHostService(os.UserConfigDir)
	s.security = sec
	return s
}

func NewHostServiceWithConfigDir(configDir func() (string, error)) *HostService {
	return newHostService(configDir)
}

func newHostService(configDir func() (string, error)) *HostService {
	return &HostService{configDir: configDir}
}

func (s *HostService) Start(ctx context.Context) error {
	dir, err := s.configDir()
	if err != nil {
		return fmt.Errorf("resolve user config directory: %w", err)
	}
	var codec storage.EncryptedPayloadCodec
	if s.security != nil {
		keys, keyErr := s.security.DerivedKeys(ctx)
		if keyErr != nil {
			if errors.Is(keyErr, security.ErrKeyringUnavailable) || errors.Is(keyErr, security.ErrAuthentication) || errors.Is(keyErr, security.ErrKeyNotFound) {

				return nil
			}
			return fmt.Errorf("load local qterm encryption key: %w", keyErr)
		}
		codec, keyErr = security.NewPayloadCipher(keys.Qterm)
		clear(keys.Vault)
		clear(keys.Qterm)
		clear(keys.Sync)
		if keyErr != nil {
			return fmt.Errorf("initialize local qterm encryption: %w", keyErr)
		}
	}
	store, err := storage.OpenWithCodec(ctx, filepath.Join(dir, "Qterm", "qterm.db"), codec)
	if err != nil {
		return fmt.Errorf("start host storage: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		_ = store.Close()
		return errors.New("host service is already started")
	}
	s.store = store
	s.reconcileSessionHistoryOnce(ctx, store)
	return nil
}

func (s *HostService) reconcileSessionHistoryOnce(ctx context.Context, store *storage.Store) {
	if s.sessionHistoryReconciled {
		return
	}
	s.sessionHistoryReconciled = true
	_, _ = store.CloseStaleSessionHistory(ctx, time.Now())
}

func (s *HostService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	err := s.store.Close()
	s.store = nil
	if err != nil {
		return fmt.Errorf("close host storage: %w", err)
	}
	return nil
}

func (s *HostService) Lock() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	err := s.store.Close()
	s.store = nil
	if err != nil {
		return fmt.Errorf("lock host storage: %w", err)
	}
	return nil
}

func (s *HostService) ListHosts() ([]model.Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, ErrHostServiceNotReady
	}
	hosts, err := s.store.ListHosts(context.Background())
	if err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "host.list_failed", "")
	}
	return hosts, nil
}

func (s *HostService) ListGroups() ([]model.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, ErrHostServiceNotReady
	}
	groups, err := s.store.ListGroups(context.Background())
	if err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "host.groups_failed", "")
	}
	return groups, nil
}

func randomID(prefix string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate random ID: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(value[:]), nil
}
