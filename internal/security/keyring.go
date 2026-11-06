package security

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	goKeyring "github.com/zalando/go-keyring"
)

const (
	DefaultKeyringService = "Qterm"
	DefaultKeyringAccount = "local-data-encryption-key"
)

type KeyringBackend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

type defaultKeyringBackend struct{}

func (defaultKeyringBackend) Get(service, account string) (string, error) {
	return goKeyring.Get(service, account)
}

func (defaultKeyringBackend) Set(service, account, secret string) error {
	return goKeyring.Set(service, account, secret)
}

func (defaultKeyringBackend) Delete(service, account string) error {
	return goKeyring.Delete(service, account)
}

type SystemKeyStore struct {
	mu      sync.Mutex
	service string
	account string
	backend KeyringBackend
}

func NewSystemKeyStore(service, account string) (*SystemKeyStore, error) {
	return NewSystemKeyStoreWithBackend(service, account, defaultKeyringBackend{})
}

func NewSystemKeyStoreWithBackend(service, account string, backend KeyringBackend) (*SystemKeyStore, error) {
	if service == "" || account == "" {
		return nil, errors.New("keyring service and account are required")
	}
	if backend == nil {
		return nil, errors.New("keyring backend is nil")
	}
	return &SystemKeyStore{
		service: service,
		account: account,
		backend: backend,
	}, nil
}

func (s *SystemKeyStore) Load(ctx context.Context) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.backend == nil {
		return nil, ErrKeyringUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	encoded, err := s.backend.Get(s.service, s.account)
	if err != nil {
		if errors.Is(err, goKeyring.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("%w: read platform keyring: %w", ErrKeyringUnavailable, err)
	}
	dek, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || !validDEK(dek) {
		clear(dek)
		return nil, ErrInvalidDEK
	}
	if err := checkContext(ctx); err != nil {
		clear(dek)
		return nil, err
	}
	return dek, nil
}

func (s *SystemKeyStore) LoadOrCreate(ctx context.Context) ([]byte, error) {
	dek, err := s.Load(ctx)
	if err == nil {
		return dek, nil
	}
	if !errors.Is(err, ErrKeyNotFound) {
		return nil, err
	}
	dek, err = GenerateDEK()
	if err != nil {
		return nil, err
	}
	if err := s.Save(ctx, dek); err != nil {
		clear(dek)
		if errors.Is(err, ErrKeyExists) {
			return s.Load(ctx)
		}
		return nil, err
	}
	return dek, nil
}

func (s *SystemKeyStore) Save(ctx context.Context, dek []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if !validDEK(dek) {
		return ErrInvalidDEK
	}
	if s == nil || s.backend == nil {
		return ErrKeyringUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	_, err := s.backend.Get(s.service, s.account)
	switch {
	case err == nil:
		return ErrKeyExists
	case !errors.Is(err, goKeyring.ErrNotFound):
		return fmt.Errorf("%w: inspect platform keyring: %w", ErrKeyringUnavailable, err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := s.backend.Set(s.service, s.account, base64.RawStdEncoding.EncodeToString(dek)); err != nil {
		return fmt.Errorf("%w: write platform keyring: %w", ErrKeyringUnavailable, err)
	}
	return nil
}

func (s *SystemKeyStore) Replace(ctx context.Context, dek []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if !validDEK(dek) {
		return ErrInvalidDEK
	}
	if s == nil || s.backend == nil {
		return ErrKeyringUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := s.backend.Set(s.service, s.account, base64.RawStdEncoding.EncodeToString(dek)); err != nil {
		return fmt.Errorf("%w: replace platform keyring value: %w", ErrKeyringUnavailable, err)
	}
	return nil
}

func (s *SystemKeyStore) Delete(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil || s.backend == nil {
		return ErrKeyringUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := s.backend.Delete(s.service, s.account); err != nil && !errors.Is(err, goKeyring.ErrNotFound) {
		return fmt.Errorf("%w: delete platform keyring value: %w", ErrKeyringUnavailable, err)
	}
	return nil
}
