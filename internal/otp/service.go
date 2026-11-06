package otp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"
)

type Service struct {
	configDir func() (string, error)
	dataKey   func(context.Context) ([]byte, error)
	now       func() time.Time

	mu    sync.Mutex
	store *SecretStore
}

func NewService(configDir func() (string, error), dataKey func(context.Context) ([]byte, error)) *Service {
	return &Service{configDir: configDir, dataKey: dataKey, now: time.Now}
}

func (s *Service) secretStore(ctx context.Context) (*SecretStore, error) {
	if s == nil || s.configDir == nil || s.dataKey == nil {
		return nil, ErrStoreUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		return s.store, nil
	}
	key, err := s.dataKey(ctx)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	dir, err := s.configDir()
	if err != nil {
		return nil, err
	}
	store, err := OpenSecretStore(dir, key)
	if err != nil {
		return nil, err
	}
	s.store = store
	return store, nil
}

func (s *Service) Save(ctx context.Context, input model.OTPSaveInput) (model.OTPEntry, error) {
	if ctx == nil {
		return model.OTPEntry{}, errors.New("otp: context is nil")
	}
	secretText := strings.TrimSpace(input.Secret)
	if secretText == "" {
		return model.OTPEntry{}, ErrInvalidSecret
	}
	entry := input.Entry
	if strings.HasPrefix(strings.ToLower(secretText), "otpauth://") {
		label, config, rawSecret, err := ParseURI(secretText)
		if err != nil {
			return model.OTPEntry{}, err
		}

		issuer, account := splitOTPLabel(label)
		if strings.TrimSpace(entry.Name) == "" {
			if issuer != "" {
				entry.Name = issuer
			} else {
				entry.Name = account
			}
		}
		if strings.TrimSpace(entry.Username) == "" && issuer != "" {
			entry.Username = account
		}
		secretText = rawSecret
		entry.Type = string(config.Type)
		entry.Algorithm = config.Algorithm
		entry.Digits = config.Digits
		entry.Period = config.Period
		entry.Counter = config.Counter
	}
	normalized, err := normalizeEntry(entry)
	if err != nil {
		return model.OTPEntry{}, err
	}
	secret, err := DecodeSecret(secretText)
	if err != nil {
		return model.OTPEntry{}, err
	}
	defer clear(secret)
	if normalized.ID == "" {
		generated, err := newEntryID()
		if err != nil {
			return model.OTPEntry{}, err
		}
		normalized.ID = generated
	}
	store, err := s.secretStore(ctx)
	if err != nil {
		return model.OTPEntry{}, err
	}
	if err := store.Put(ctx, normalized.ID, secret); err != nil {
		return model.OTPEntry{}, err
	}
	return normalized, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	store, err := s.secretStore(ctx)
	if err != nil {

		if errors.Is(err, ErrStoreUnavailable) {
			return nil
		}
		return err
	}
	return store.Delete(ctx, id)
}

func (s *Service) Generate(ctx context.Context, entry model.OTPEntry) (model.OTPCode, error) {
	normalized, err := normalizeEntry(entry)
	if err != nil {
		return model.OTPCode{}, err
	}
	store, err := s.secretStore(ctx)
	if err != nil {
		return model.OTPCode{}, err
	}
	secret, err := store.Get(ctx, normalized.ID)
	if err != nil {
		return model.OTPCode{}, err
	}
	defer clear(secret)
	config := Config{
		Type:      Type(normalized.Type),
		Algorithm: normalized.Algorithm,
		Digits:    normalized.Digits,
		Period:    normalized.Period,
		Counter:   normalized.Counter,
	}
	now := s.now()
	code, err := Generate(config, secret, now)
	if err != nil {
		return model.OTPCode{}, err
	}
	normalized, err = normalizeEntry(normalized)
	if err != nil {
		return model.OTPCode{}, err
	}
	return model.OTPCode{
		Code:             code,
		RemainingSeconds: RemainingSeconds(now, normalized.Period),
		Counter:          normalized.Counter,
	}, nil
}

func (s *Service) ProvisioningURI(ctx context.Context, entry model.OTPEntry) (string, error) {
	normalized, err := normalizeEntry(entry)
	if err != nil {
		return "", err
	}
	store, err := s.secretStore(ctx)
	if err != nil {
		return "", err
	}
	secret, err := store.Get(ctx, normalized.ID)
	if err != nil {
		return "", err
	}
	defer clear(secret)

	label := normalized.Name
	if label == "" {
		label = "Qterm"
	}
	if username := strings.TrimSpace(normalized.Username); username != "" {
		label = label + ":" + username
	}
	config := Config{
		Type:      Type(normalized.Type),
		Algorithm: normalized.Algorithm,
		Digits:    normalized.Digits,
		Period:    normalized.Period,
		Counter:   normalized.Counter,
	}
	return BuildURI(config, label, EncodeSecret(secret)), nil
}

func normalizeEntry(entry model.OTPEntry) (model.OTPEntry, error) {
	config, err := Config{
		Type:      Type(entry.Type),
		Algorithm: entry.Algorithm,
		Digits:    entry.Digits,
		Period:    entry.Period,
		Counter:   entry.Counter,
	}.Normalize()
	if err != nil {
		return model.OTPEntry{}, err
	}
	result := entry
	result.ID = strings.TrimSpace(entry.ID)
	result.Name = strings.TrimSpace(entry.Name)
	result.Username = strings.TrimSpace(entry.Username)
	result.HostID = strings.TrimSpace(entry.HostID)
	result.Type = string(config.Type)
	result.Algorithm = config.Algorithm
	result.Digits = config.Digits
	result.Period = config.Period
	result.Counter = config.Counter
	if result.Type == string(TypeHOTP) && result.Period == 0 {
		result.Period = DefaultPeriod
	}
	return result, nil
}

func splitOTPLabel(label string) (issuer, account string) {
	value := strings.TrimSpace(label)
	if index := strings.Index(value, ":"); index > 0 {
		issuer = strings.TrimSpace(value[:index])
		account = strings.TrimSpace(value[index+1:])
		return issuer, account
	}
	return "", value
}

func newEntryID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "otp-" + hex.EncodeToString(buffer), nil
}
