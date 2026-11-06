package security

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type CustomKeyStore struct {
	mu     sync.Mutex
	path   string
	random io.Reader
}

func NewCustomKeyStore(path string) (*CustomKeyStore, error) {
	return newCustomKeyStore(path, nil)
}

func newCustomKeyStore(path string, random io.Reader) (*CustomKeyStore, error) {
	if err := validateSecurityPath(path); err != nil {
		return nil, err
	}
	if random == nil {
		random = rand.Reader
	}
	return &CustomKeyStore{path: filepath.Clean(path), random: random}, nil
}

func (s *CustomKeyStore) Load(ctx context.Context, customKey []byte) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := validateCustomKey(customKey); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrKeyNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	envelope, err := readSecurityEnvelope(s.path, customEnvelopeMagic)
	if err != nil {
		return nil, err
	}
	defer clear(envelope)
	dek, err := UnwrapDEKWithCustomKey(envelope, customKey)
	if err != nil {
		clear(dek)
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		clear(dek)
		return nil, err
	}
	return dek, nil
}

func (s *CustomKeyStore) Exists(ctx context.Context) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	if s == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect custom encryption key envelope: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(maxEnvelopeSize) {
		return false, ErrInvalidEnvelope
	}
	envelope, err := readSecurityEnvelope(s.path, customEnvelopeMagic)
	if err != nil {
		return false, err
	}
	clear(envelope)
	return true, nil
}

func (s *CustomKeyStore) Save(ctx context.Context, customKey, dek []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateCustomKey(customKey); err != nil {
		return err
	}
	if !validDEK(dek) {
		return ErrInvalidDEK
	}
	if s == nil || s.random == nil {
		return ErrKeyNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	envelope, err := wrapDEKWithKey(dek, customKey, customEnvelopeMagic, customKeyAADLabel, s.random)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return writeSecurityEnvelopeCreateOnly(ctx, s.path, envelope, customEnvelopeMagic)
}

func (s *CustomKeyStore) Replace(ctx context.Context, customKey, dek []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateCustomKey(customKey); err != nil {
		return err
	}
	if !validDEK(dek) {
		return ErrInvalidDEK
	}
	if s == nil || s.random == nil {
		return ErrKeyNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	envelope, err := wrapDEKWithKey(dek, customKey, customEnvelopeMagic, customKeyAADLabel, s.random)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return replaceSecurityEnvelope(ctx, s.path, envelope, customEnvelopeMagic)
}

func (s *CustomKeyStore) Rotate(ctx context.Context, currentKey, newKey []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateCustomKey(currentKey); err != nil {
		return err
	}
	if err := validateCustomKey(newKey); err != nil {
		return err
	}
	if s == nil || s.random == nil {
		return ErrKeyNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	envelope, err := readSecurityEnvelope(s.path, customEnvelopeMagic)
	if err != nil {
		return err
	}
	dek, err := UnwrapDEKWithCustomKey(envelope, currentKey)
	clear(envelope)
	if err != nil {
		clear(dek)
		return err
	}
	defer clear(dek)
	if err := checkContext(ctx); err != nil {
		return err
	}
	rotated, err := wrapDEKWithKey(dek, newKey, customEnvelopeMagic, customKeyAADLabel, s.random)
	if err != nil {
		return err
	}
	defer clear(rotated)
	return replaceSecurityEnvelope(ctx, s.path, rotated, customEnvelopeMagic)
}

func (s *CustomKeyStore) Delete(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	err := os.Remove(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete custom encryption key envelope: %w", err)
	}
	return syncSecurityDirectory(filepath.Dir(s.path))
}

func validateSecurityPath(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return ErrInvalidPath
	}
	cleanPath := filepath.Clean(path)
	dir := filepath.Dir(cleanPath)
	volumeRoot := filepath.Clean(filepath.VolumeName(dir) + string(os.PathSeparator))
	if cleanPath == dir || dir == volumeRoot {
		return ErrInvalidPath
	}
	return nil
}

func readSecurityEnvelope(path, expectedMagic string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("inspect security envelope: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(maxEnvelopeSize) {
		return nil, ErrInvalidEnvelope
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open security envelope: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat security envelope: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, ErrInvalidEnvelope
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(maxEnvelopeSize+1)))
	if err != nil {
		return nil, fmt.Errorf("read security envelope: %w", err)
	}
	if len(raw) != maxEnvelopeSize {
		clear(raw)
		return nil, ErrInvalidEnvelope
	}
	if _, err := parseEnvelope(raw, expectedMagic); err != nil {
		clear(raw)
		return nil, err
	}
	if runtime.GOOS != "windows" {

		if err := file.Chmod(0o600); err != nil {
			clear(raw)
			return nil, fmt.Errorf("protect security envelope: %w", err)
		}
	}
	return raw, nil
}

func writeSecurityEnvelopeCreateOnly(ctx context.Context, path string, envelope []byte, expectedMagic string) error {
	if len(envelope) != maxEnvelopeSize {
		return ErrInvalidEnvelope
	}
	if _, err := parseEnvelope(envelope, expectedMagic); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := prepareSecurityDirectory(dir); err != nil {
		return err
	}
	temporary, temporaryPath, err := createSecurityTemporary(dir, envelope)
	if err != nil {
		return err
	}
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrKeyExists
		}
		return fmt.Errorf("commit security envelope: %w", err)
	}
	if err := syncSecurityDirectory(dir); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close security envelope: %w", err)
	}
	keepTemporary = false
	return nil
}

func replaceSecurityEnvelope(ctx context.Context, path string, envelope []byte, expectedMagic string) error {
	if len(envelope) != maxEnvelopeSize {
		return ErrInvalidEnvelope
	}
	if _, err := parseEnvelope(envelope, expectedMagic); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := prepareSecurityDirectory(dir); err != nil {
		return err
	}
	temporary, temporaryPath, err := createSecurityTemporary(dir, envelope)
	if err != nil {
		return err
	}
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close security envelope: %w", err)
	}
	if err := replaceFileAtomically(temporaryPath, path); err != nil {
		return fmt.Errorf("commit security envelope: %w", err)
	}
	keepTemporary = false
	if err := syncSecurityDirectory(dir); err != nil {
		return err
	}
	return nil
}

func prepareSecurityDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create security directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("protect security directory: %w", err)
		}
	}
	return nil
}

func createSecurityTemporary(dir string, envelope []byte) (*os.File, string, error) {
	temporary, err := os.CreateTemp(dir, ".qterm-security-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temporary security envelope: %w", err)
	}
	temporaryPath := temporary.Name()
	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
			return nil, "", fmt.Errorf("protect temporary security envelope: %w", err)
		}
	}
	written, err := temporary.Write(envelope)
	if err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return nil, "", fmt.Errorf("write temporary security envelope: %w", err)
	}
	if written != len(envelope) {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return nil, "", io.ErrShortWrite
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return nil, "", fmt.Errorf("sync temporary security envelope: %w", err)
	}
	return temporary, temporaryPath, nil
}
