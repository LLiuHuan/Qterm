package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

const (
	maxProxyNameLength = 80

	maxProxyHostLength     = 1024
	maxProxyUsernameLength = 1024
	maxProxyIDLength       = 1024
)

type StoreAccess func(fn func(*storage.Store) error) error

type Service struct {
	store StoreAccess
}

func NewService(store StoreAccess) *Service {
	return &Service{store: store}
}

func (s *Service) ListProxies() ([]model.ProxyRecord, error) {
	var proxies []model.ProxyRecord
	if err := s.withStore(context.Background(), "proxy.read_failed", func(ctx context.Context, store *storage.Store) error {
		var err error
		proxies, err = store.ListProxies(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	if proxies == nil {
		proxies = make([]model.ProxyRecord, 0)
	}
	return proxies, nil
}

func (s *Service) ProxyByID(ctx context.Context, id string) (model.ProxyRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.ProxyRecord{}, errno.New(errno.CodeInvalidArg, "proxy.id_required", "")
	}
	if len(id) > maxProxyIDLength {
		return model.ProxyRecord{}, errno.New(errno.CodeInvalidArg, "proxy.id_invalid", "")
	}
	var record model.ProxyRecord
	if err := s.withStore(ctx, "proxy.read_failed", func(callCtx context.Context, store *storage.Store) error {
		var err error
		record, err = store.ProxyByID(callCtx, id)
		return err
	}); err != nil {
		return model.ProxyRecord{}, err
	}
	return record, nil
}

func (s *Service) CreateProxy(input model.ProxyInput) (model.ProxyRecord, error) {
	normalized, err := normalizeProxyInput(input)
	if err != nil {
		return model.ProxyRecord{}, err
	}
	if err := validateProxyAuthentication(normalized, false); err != nil {
		return model.ProxyRecord{}, err
	}
	id, err := newProxyID()
	if err != nil {
		return model.ProxyRecord{}, errno.New(errno.CodeUnknown, "proxy.id_failed", "")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := model.ProxyRecord{
		ID:        id,
		Name:      normalized.Name,
		Mode:      normalized.Mode,
		Host:      normalized.Host,
		Port:      normalized.Port,
		Username:  normalized.Username,
		CreatedAt: now,
		UpdatedAt: now,
	}
	password := []byte(normalized.Password)
	defer clear(password)
	if err := s.withStore(context.Background(), "proxy.save_failed", func(ctx context.Context, store *storage.Store) error {
		var err error
		record, err = store.CreateProxyWithPassword(ctx, record, password)
		return err
	}); err != nil {
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return model.ProxyRecord{}, errno.New(errno.CodePermissionDenied, "proxy.password_unavailable", "")
		}
		return model.ProxyRecord{}, err
	}
	return record, nil
}

func (s *Service) UpdateProxy(id string, input model.ProxyInput) (model.ProxyRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.ProxyRecord{}, errno.New(errno.CodeInvalidArg, "proxy.id_required", "")
	}
	if len(id) > maxProxyIDLength {
		return model.ProxyRecord{}, errno.New(errno.CodeInvalidArg, "proxy.id_invalid", "")
	}
	existing, err := s.ProxyByID(context.Background(), id)
	if err != nil {
		return model.ProxyRecord{}, err
	}
	normalized, err := normalizeProxyInput(input)
	if err != nil {
		return model.ProxyRecord{}, err
	}
	preservesDirectPassword := normalized.Password == "" && !normalized.ClearPassword && existing.PasswordConfigured
	if err := validateProxyAuthentication(normalized, preservesDirectPassword); err != nil {
		return model.ProxyRecord{}, err
	}
	record := model.ProxyRecord{
		ID:        id,
		Name:      normalized.Name,
		Mode:      normalized.Mode,
		Host:      normalized.Host,
		Port:      normalized.Port,
		Username:  normalized.Username,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	password := []byte(normalized.Password)
	defer clear(password)
	if err := s.withStore(context.Background(), "proxy.save_failed", func(ctx context.Context, store *storage.Store) error {
		var err error
		record, err = store.UpdateProxyWithPassword(ctx, record, password, normalized.ClearPassword)
		return err
	}); err != nil {
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return model.ProxyRecord{}, errno.New(errno.CodePermissionDenied, "proxy.password_unavailable", "")
		}
		return model.ProxyRecord{}, err
	}
	return record, nil
}

func (s *Service) UseProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errno.New(errno.CodeInvalidArg, "proxy.id_required", "")
	}
	if len(id) > maxProxyIDLength {
		return errno.New(errno.CodeInvalidArg, "proxy.id_invalid", "")
	}
	if use == nil {
		return errno.New(errno.CodeInvalidArg, "proxy.password_unavailable", "")
	}
	err := s.withStore(ctx, "proxy.password_read_failed", func(callCtx context.Context, store *storage.Store) error {
		return store.UseProxyPassword(callCtx, id, use)
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, model.ErrCredentialVaultLocked) {
		return errno.New(errno.CodePermissionDenied, "proxy.password_unavailable", "")
	}
	if errno.Extract(err) != nil {
		return err
	}
	return errno.New(errno.CodeStoreFailed, "proxy.password_read_failed", "")
}

func (s *Service) DeleteProxy(id string) (int, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, errno.New(errno.CodeInvalidArg, "proxy.id_required", "")
	}
	if len(id) > maxProxyIDLength {
		return 0, errno.New(errno.CodeInvalidArg, "proxy.id_invalid", "")
	}
	referenced := 0
	if err := s.withStore(context.Background(), "proxy.delete_failed", func(ctx context.Context, store *storage.Store) error {
		var err error
		referenced, err = store.DeleteProxy(ctx, id)
		return err
	}); err != nil {
		return 0, err
	}
	return referenced, nil
}

func (s *Service) withStore(ctx context.Context, failureKey string, fn func(context.Context, *storage.Store) error) error {
	if s == nil || s.store == nil {
		return errno.New(errno.CodeSessionClosed, "proxy.not_ready", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var operationErr error
	err := s.store(func(store *storage.Store) error {
		operationErr = fn(ctx, store)
		return operationErr
	})
	if err == nil {
		return nil
	}

	if errno.Extract(err) != nil {
		return err
	}
	if errors.Is(err, storage.ErrNotFound) || errors.Is(operationErr, storage.ErrNotFound) {
		return errno.New(errno.CodeNotFound, "proxy.not_found", "")
	}
	if errors.Is(err, model.ErrCredentialVaultLocked) || errors.Is(operationErr, model.ErrCredentialVaultLocked) {
		return model.ErrCredentialVaultLocked
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errno.New(errno.CodeStoreFailed, failureKey, "")
}

func normalizeProxyInput(input model.ProxyInput) (model.ProxyInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.name_required", "")
	}
	if utf8.RuneCountInString(input.Name) > maxProxyNameLength {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.name_too_long", "")
	}
	input.Mode = strings.TrimSpace(input.Mode)

	if input.Mode != model.ProxyModeSOCKS5 && input.Mode != model.ProxyModeHTTP {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.mode_invalid", "")
	}
	input.Host = strings.TrimSpace(input.Host)
	if input.Host == "" {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.host_required", "")
	}
	if len(input.Host) > maxProxyHostLength || !utf8.ValidString(input.Host) || strings.IndexByte(input.Host, 0) >= 0 {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.host_invalid", "")
	}
	if input.Port < 1 || input.Port > 65535 {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.port_invalid", "")
	}
	input.Username = strings.TrimSpace(input.Username)
	if len(input.Username) > maxProxyUsernameLength ||
		!utf8.ValidString(input.Username) ||
		strings.IndexByte(input.Username, 0) >= 0 {
		return model.ProxyInput{}, errno.New(errno.CodeInvalidArg, "proxy.username_invalid", "")
	}
	return input, nil
}

func validateProxyAuthentication(input model.ProxyInput, preservesDirectPassword bool) error {
	hasPassword := input.Password != ""
	if (hasPassword || preservesDirectPassword) && input.Username == "" {
		return errno.New(errno.CodeInvalidArg, "proxy.username_required", "")
	}
	return nil
}

func newProxyID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate proxy ID: %w", err)
	}
	return "proxy_" + hex.EncodeToString(value[:]), nil
}
