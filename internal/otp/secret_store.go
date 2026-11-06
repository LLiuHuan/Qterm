package otp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type SecretStore struct {
	path string
	key  []byte
	mu   sync.Mutex
}

const secretStoreVersion = 1

type secretFile struct {
	Version int               `json:"version"`
	Secrets map[string]string `json:"secrets"`
}

var (
	ErrSecretNotFound = errors.New("otp: secret not found")

	ErrStoreUnavailable = errors.New("otp: secret store unavailable")
)

func OpenSecretStore(configDir string, dataKey []byte) (*SecretStore, error) {
	if configDir == "" || len(dataKey) < 16 {
		return nil, ErrStoreUnavailable
	}

	key := make([]byte, 32)
	copy(key, dataKey)
	return &SecretStore{path: filepath.Join(configDir, "otp-secrets.json"), key: key}, nil
}

func (s *SecretStore) Close() {
	if s == nil {
		return
	}
	clear(s.key)
}

func (s *SecretStore) Put(ctx context.Context, id string, secret []byte) error {
	if s == nil || len(s.key) == 0 {
		return ErrStoreUnavailable
	}
	if id == "" || len(secret) == 0 {
		return errors.New("otp: empty secret")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sealed, err := s.seal(id, secret)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return err
	}
	file.Secrets[id] = base64.StdEncoding.EncodeToString(sealed)
	return s.saveLocked(file)
}

func (s *SecretStore) Get(ctx context.Context, id string) ([]byte, error) {
	if s == nil || len(s.key) == 0 {
		return nil, ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	file, err := s.loadLocked()
	encoded, exists := file.Secrets[id]
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrSecretNotFound
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrSecretNotFound
	}
	return s.open(id, raw)
}

func (s *SecretStore) Delete(ctx context.Context, id string) error {
	if s == nil || len(s.key) == 0 {
		return ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := file.Secrets[id]; !exists {
		return nil
	}
	delete(file.Secrets, id)
	return s.saveLocked(file)
}

func (s *SecretStore) Has(ctx context.Context, id string) bool {
	_, err := s.Get(ctx, id)
	return err == nil
}

func (s *SecretStore) seal(id string, secret []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, secret, []byte(id)), nil
}

func (s *SecretStore) open(id string, payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < gcm.NonceSize() {
		return nil, ErrSecretNotFound
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(id))
	if err != nil {

		return nil, fmt.Errorf("otp: decrypt secret: %w", err)
	}
	return plain, nil
}

func (s *SecretStore) loadLocked() (secretFile, error) {
	file := secretFile{Version: secretStoreVersion, Secrets: map[string]string{}}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return file, nil
		}
		return file, err
	}
	if len(raw) == 0 {
		return file, nil
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return secretFile{Version: secretStoreVersion, Secrets: map[string]string{}}, nil
	}
	if file.Secrets == nil {
		file.Secrets = map[string]string{}
	}
	file.Version = secretStoreVersion
	return file, nil
}

func (s *SecretStore) saveLocked(file secretFile) error {
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".otp-secrets-*")
	if err != nil {
		return err
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
	return os.Rename(tempName, s.path)
}
