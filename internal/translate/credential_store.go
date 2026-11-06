package translate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"changeme/internal/security"
)

const (
	credentialStoreVersion = 1
	credentialStoreDomain  = "translation-provider-credentials"
)

var (
	ErrCredentialStoreUnavailable = errors.New("translate: credential store unavailable")

	ErrCredentialNotFound = errors.New("translate: credential not found")

	ErrInvalidCredentialProvider = errors.New("translate: invalid credential provider")

	ErrInvalidCredentials = errors.New("translate: invalid credentials")

	ErrCredentialStoreCorrupt = errors.New("translate: credential store is corrupt")
)

type CredentialInput struct {
	APIKey string `json:"apiKey"`
	AppID  string `json:"appId"`
	AppKey string `json:"appKey"`
}

type CredentialStatus struct {
	Provider   Provider `json:"provider"`
	Configured bool     `json:"configured"`
}

type CredentialStore struct {
	path   string
	cipher *security.PayloadCipher
	mu     sync.Mutex
}

type credentialFile struct {
	Version     int               `json:"version"`
	Credentials map[string]string `json:"credentials"`
}

type credentialRecord struct {
	APIKey string `json:"apiKey,omitempty"`
	AppID  string `json:"appId,omitempty"`
	AppKey string `json:"appKey,omitempty"`
}

func SupportedCredentialProviders() []Provider {
	return []Provider{ProviderDeepL, ProviderBaidu, ProviderAli, ProviderYoudao}
}

func OpenCredentialStore(root string, dataKey []byte) (*CredentialStore, error) {
	if root == "" || !filepath.IsAbs(root) || len(dataKey) == 0 {
		return nil, ErrCredentialStoreUnavailable
	}
	codec, err := security.NewPayloadCipher(dataKey)
	if err != nil {
		return nil, fmt.Errorf("translate: create credential cipher: %w", err)
	}
	return &CredentialStore{
		path:   filepath.Join(filepath.Clean(root), "translation-credentials.json"),
		cipher: codec,
	}, nil
}

func (s *CredentialStore) Put(ctx context.Context, provider Provider, credentials Credentials) error {
	if err := checkCredentialContext(ctx); err != nil {
		return err
	}
	if err := validateCredentials(provider, credentials); err != nil {
		return err
	}
	record, err := json.Marshal(credentialRecord{
		APIKey: strings.TrimSpace(credentials.APIKey),
		AppID:  strings.TrimSpace(credentials.AppID),
		AppKey: strings.TrimSpace(credentials.AppKey),
	})
	if err != nil {
		return fmt.Errorf("translate: encode credential: %w", err)
	}
	defer clear(record)
	sealed, err := s.seal(provider, record)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return err
	}
	file.Credentials[string(provider)] = base64.StdEncoding.EncodeToString(sealed)
	clear(sealed)
	return s.saveLocked(file)
}

func (s *CredentialStore) Get(ctx context.Context, provider Provider) (Credentials, error) {
	if err := checkCredentialContext(ctx); err != nil {
		return Credentials{}, err
	}
	if err := validateCredentialProvider(provider); err != nil {
		return Credentials{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return Credentials{}, err
	}
	encoded, exists := file.Credentials[string(provider)]
	if !exists || strings.TrimSpace(encoded) == "" {
		return Credentials{}, ErrCredentialNotFound
	}
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Credentials{}, fmt.Errorf("%w: invalid base64", ErrCredentialStoreCorrupt)
	}
	defer clear(sealed)
	plain, err := s.open(provider, sealed)
	if err != nil {
		return Credentials{}, err
	}
	defer clear(plain)
	var record credentialRecord
	if err := json.Unmarshal(plain, &record); err != nil {
		return Credentials{}, fmt.Errorf("%w: invalid record", ErrCredentialStoreCorrupt)
	}
	credentials := Credentials{
		APIKey: strings.TrimSpace(record.APIKey),
		AppID:  strings.TrimSpace(record.AppID),
		AppKey: strings.TrimSpace(record.AppKey),
	}
	if err := validateCredentials(provider, credentials); err != nil {
		return Credentials{}, fmt.Errorf("%w: invalid record", ErrCredentialStoreCorrupt)
	}
	return credentials, nil
}

func (s *CredentialStore) Delete(ctx context.Context, provider Provider) error {
	if err := checkCredentialContext(ctx); err != nil {
		return err
	}
	if err := validateCredentialProvider(provider); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := file.Credentials[string(provider)]; !exists {
		return nil
	}
	delete(file.Credentials, string(provider))
	return s.saveLocked(file)
}

func (s *CredentialStore) Status(ctx context.Context) ([]CredentialStatus, error) {
	if err := checkCredentialContext(ctx); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	statuses := make([]CredentialStatus, 0, len(SupportedCredentialProviders()))
	for _, provider := range SupportedCredentialProviders() {
		encoded, exists := file.Credentials[string(provider)]
		if exists && strings.TrimSpace(encoded) != "" {
			sealed, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil {
				return nil, fmt.Errorf("%w: invalid base64", ErrCredentialStoreCorrupt)
			}
			plain, openErr := s.open(provider, sealed)
			clear(sealed)
			if openErr != nil {
				return nil, openErr
			}
			var record credentialRecord
			decodeErr = json.Unmarshal(plain, &record)
			clear(plain)
			if decodeErr != nil {
				return nil, fmt.Errorf("%w: invalid record", ErrCredentialStoreCorrupt)
			}
			if validateErr := validateCredentials(provider, Credentials{
				APIKey: record.APIKey,
				AppID:  record.AppID,
				AppKey: record.AppKey,
			}); validateErr != nil {
				return nil, fmt.Errorf("%w: invalid record", ErrCredentialStoreCorrupt)
			}
		}
		statuses = append(statuses, CredentialStatus{Provider: provider, Configured: exists && strings.TrimSpace(encoded) != ""})
	}
	return statuses, nil
}

func (s *CredentialStore) seal(provider Provider, plaintext []byte) ([]byte, error) {
	if s == nil || s.cipher == nil {
		return nil, ErrCredentialStoreUnavailable
	}
	sealed, err := s.cipher.Seal(credentialStoreDomain, string(provider), plaintext)
	if err != nil {
		return nil, fmt.Errorf("translate: encrypt credential: %w", err)
	}
	return sealed, nil
}

func (s *CredentialStore) open(provider Provider, sealed []byte) ([]byte, error) {
	if s == nil || s.cipher == nil {
		return nil, ErrCredentialStoreUnavailable
	}
	plain, err := s.cipher.Open(credentialStoreDomain, string(provider), sealed)
	if err != nil {
		return nil, fmt.Errorf("%w: authentication failed", ErrCredentialStoreCorrupt)
	}
	return plain, nil
}

func (s *CredentialStore) loadLocked() (credentialFile, error) {
	file := credentialFile{Version: credentialStoreVersion, Credentials: map[string]string{}}
	if s == nil || s.path == "" {
		return file, ErrCredentialStoreUnavailable
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return file, nil
		}
		return file, fmt.Errorf("translate: read credential store: %w", err)
	}
	if len(raw) == 0 {
		return file, nil
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return file, fmt.Errorf("%w: invalid json", ErrCredentialStoreCorrupt)
	}
	if file.Version != credentialStoreVersion {
		return file, fmt.Errorf("%w: unsupported version", ErrCredentialStoreCorrupt)
	}
	if file.Credentials == nil {
		file.Credentials = map[string]string{}
	}
	return file, nil
}

func (s *CredentialStore) saveLocked(file credentialFile) error {
	if s == nil || s.path == "" {
		return ErrCredentialStoreUnavailable
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("translate: encode credential store: %w", err)
	}
	defer clear(raw)
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("translate: create credential store directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".translation-credentials-*")
	if err != nil {
		return fmt.Errorf("translate: create credential store temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("translate: replace credential store: %w", err)
	}
	return nil
}

func checkCredentialContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("translate: context is nil")
	}
	return ctx.Err()
}

func validateCredentialProvider(provider Provider) error {
	switch provider {
	case ProviderDeepL, ProviderBaidu, ProviderAli, ProviderYoudao:
		return nil
	default:
		return ErrInvalidCredentialProvider
	}
}

func validateCredentials(provider Provider, credentials Credentials) error {
	if err := validateCredentialProvider(provider); err != nil {
		return err
	}
	switch provider {
	case ProviderDeepL:
		if strings.TrimSpace(credentials.APIKey) == "" {
			return ErrInvalidCredentials
		}
	case ProviderBaidu, ProviderAli, ProviderYoudao:
		if strings.TrimSpace(credentials.AppID) == "" || strings.TrimSpace(credentials.AppKey) == "" {
			return ErrInvalidCredentials
		}
	}
	return nil
}
