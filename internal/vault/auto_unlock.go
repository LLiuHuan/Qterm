package vault

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
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

const (
	autoUnlockKeySize       = MasterKeySize
	autoUnlockWrapMagic     = "QTAW"
	autoUnlockWrapVersion   = byte(1)
	autoUnlockWrapAlgorithm = byte(1)
	autoUnlockNonceSize     = 12
	autoUnlockTagSize       = 16
	autoUnlockHeaderSize    = len(autoUnlockWrapMagic) + 2
	autoUnlockEnvelopeSize  = autoUnlockHeaderSize + autoUnlockNonceSize + MasterKeySize + autoUnlockTagSize
	autoUnlockAAD           = "qterm-vault-auto-unlock"
)

var (
	ErrInvalidAutoUnlockKey     = errors.New("invalid vault auto-unlock key")
	ErrInvalidAutoUnlockWrap    = errors.New("invalid vault auto-unlock wrapping file")
	ErrAutoUnlockAuthentication = errors.New("vault auto-unlock authentication failed")
)

type AutoUnlockStore struct {
	mu     sync.Mutex
	path   string
	key    []byte
	random io.Reader
	closed bool
}

var _ MasterKeyStore = (*AutoUnlockStore)(nil)

func NewAutoUnlockStore(path string, key []byte) (*AutoUnlockStore, error) {
	if err := validatePasswordKeyPath(path); err != nil {
		return nil, err
	}
	if len(key) != autoUnlockKeySize {
		return nil, ErrInvalidAutoUnlockKey
	}
	return &AutoUnlockStore{
		path:   filepath.Clean(path),
		key:    bytes.Clone(key),
		random: rand.Reader,
	}, nil
}

func (s *AutoUnlockStore) Load(ctx context.Context) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrMasterKeyUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if s.closed || len(s.key) != autoUnlockKeySize {
		return nil, ErrMasterKeyUnavailable
	}
	envelope, err := readAutoUnlockEnvelope(s.path)
	if err != nil {
		return nil, err
	}
	defer clear(envelope)
	masterKey, err := unwrapAutoUnlockMasterKey(s.key, envelope)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		clear(masterKey)
		return nil, err
	}
	return masterKey, nil
}

func (s *AutoUnlockStore) Save(ctx context.Context, masterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(masterKey) != MasterKeySize {
		return ErrInvalidKey
	}
	if s == nil {
		return ErrMasterKeyUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if s.closed || len(s.key) != autoUnlockKeySize || s.random == nil {
		return ErrMasterKeyUnavailable
	}
	envelope, err := wrapAutoUnlockMasterKey(s.key, masterKey, s.random)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return writeAutoUnlockEnvelopeCreateOnly(s.path, envelope)
}

func (s *AutoUnlockStore) Replace(ctx context.Context, masterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(masterKey) != MasterKeySize {
		return ErrInvalidKey
	}
	if s == nil {
		return ErrMasterKeyUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if s.closed || len(s.key) != autoUnlockKeySize || s.random == nil {
		return ErrMasterKeyUnavailable
	}
	envelope, err := wrapAutoUnlockMasterKey(s.key, masterKey, s.random)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return replaceAutoUnlockEnvelope(s.path, envelope)
}

func (s *AutoUnlockStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	clear(s.key)
	s.key = nil
	s.random = nil
	s.closed = true
	return nil
}

func LoadOrCreateAutoUnlockKey(path string) ([]byte, error) {
	key, _, err := LoadOrCreateAutoUnlockKeyWithStatus(path)
	return key, err
}

func LoadOrCreateAutoUnlockKeyWithStatus(path string) ([]byte, bool, error) {
	if err := validatePasswordKeyPath(path); err != nil {
		return nil, false, err
	}
	if key, err := LoadAutoUnlockKey(path); err == nil {
		return key, false, nil
	} else if !errors.Is(err, ErrMasterKeyNotFound) {
		return nil, false, err
	}

	key := make([]byte, autoUnlockKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		clear(key)
		return nil, false, fmt.Errorf("generate auto unlock key: %w", err)
	}
	if err := writeAutoUnlockKeyCreateOnly(path, key); err != nil {
		clear(key)
		if errors.Is(err, ErrMasterKeyExists) {
			winner, loadErr := LoadAutoUnlockKey(path)
			return winner, false, loadErr
		}
		return nil, false, err
	}
	return key, true, nil
}

func LoadAutoUnlockKey(path string) ([]byte, error) {
	if err := validatePasswordKeyPath(path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("inspect auto unlock key: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != autoUnlockKeySize {
		return nil, ErrInvalidAutoUnlockKey
	}
	if err := protectLocalPath(path, false); err != nil {
		return nil, fmt.Errorf("protect auto unlock key: %w", err)
	}
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open auto unlock key: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened auto unlock key: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, ErrMasterKeyDenied
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return nil, fmt.Errorf("protect auto unlock key: %w", err)
		}
	}
	key, err := io.ReadAll(io.LimitReader(file, autoUnlockKeySize+1))
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("read auto unlock key: %w", err)
	}
	if len(key) != autoUnlockKeySize {
		clear(key)
		return nil, ErrInvalidAutoUnlockKey
	}
	return key, nil
}

func wrapAutoUnlockMasterKey(key, masterKey []byte, random io.Reader) ([]byte, error) {
	if len(key) != autoUnlockKeySize || len(masterKey) != MasterKeySize {
		return nil, ErrInvalidKey
	}
	if random == nil {
		return nil, ErrMasterKeyUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create auto unlock cipher: %w", err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, autoUnlockNonceSize)
	if err != nil {
		return nil, fmt.Errorf("create auto unlock AEAD: %w", err)
	}
	nonce := make([]byte, autoUnlockNonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		clear(nonce)
		return nil, fmt.Errorf("generate auto unlock nonce: %w", err)
	}
	header := []byte(autoUnlockWrapMagic + string([]byte{autoUnlockWrapVersion, autoUnlockWrapAlgorithm}))
	sealed := aead.Seal(nil, nonce, masterKey, append([]byte(autoUnlockAAD), header...))
	envelope := make([]byte, 0, autoUnlockEnvelopeSize)
	envelope = append(envelope, header...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, sealed...)
	clear(nonce)
	clear(sealed)
	return envelope, nil
}

func unwrapAutoUnlockMasterKey(key, envelope []byte) ([]byte, error) {
	if len(key) != autoUnlockKeySize || len(envelope) != autoUnlockEnvelopeSize {
		return nil, ErrInvalidAutoUnlockWrap
	}
	header := envelope[:autoUnlockHeaderSize]
	if string(header[:len(autoUnlockWrapMagic)]) != autoUnlockWrapMagic ||
		header[len(autoUnlockWrapMagic)] != autoUnlockWrapVersion ||
		header[len(autoUnlockWrapMagic)+1] != autoUnlockWrapAlgorithm {
		return nil, ErrUnsupportedMasterKeyWrap
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrMasterKeyUnavailable
	}
	aead, err := cipher.NewGCMWithNonceSize(block, autoUnlockNonceSize)
	if err != nil {
		return nil, ErrMasterKeyUnavailable
	}
	nonceStart := autoUnlockHeaderSize
	nonceEnd := nonceStart + autoUnlockNonceSize
	masterKey, err := aead.Open(nil, envelope[nonceStart:nonceEnd], envelope[nonceEnd:], append([]byte(autoUnlockAAD), header...))
	if err != nil {
		return nil, ErrAutoUnlockAuthentication
	}
	if len(masterKey) != MasterKeySize {
		clear(masterKey)
		return nil, ErrInvalidAutoUnlockWrap
	}
	return masterKey, nil
}

func writeAutoUnlockKeyCreateOnly(path string, key []byte) error {
	if len(key) != autoUnlockKeySize {
		return ErrInvalidAutoUnlockKey
	}
	return writeAutoUnlockFileCreateOnly(path, key, "key")
}

func readAutoUnlockEnvelope(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("inspect auto unlock wrapper: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(autoUnlockEnvelopeSize) {
		return nil, ErrInvalidAutoUnlockWrap
	}
	if err := protectLocalPath(path, false); err != nil {
		return nil, fmt.Errorf("protect auto unlock wrapper: %w", err)
	}
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open auto unlock wrapper: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened auto unlock wrapper: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, ErrMasterKeyDenied
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return nil, fmt.Errorf("protect auto unlock wrapper: %w", err)
		}
	}
	envelope, err := io.ReadAll(io.LimitReader(file, int64(autoUnlockEnvelopeSize+1)))
	if err != nil {
		clear(envelope)
		return nil, fmt.Errorf("read auto unlock wrapper: %w", err)
	}
	if len(envelope) != autoUnlockEnvelopeSize {
		clear(envelope)
		return nil, ErrInvalidAutoUnlockWrap
	}
	return envelope, nil
}

func writeAutoUnlockEnvelopeCreateOnly(path string, envelope []byte) error {
	if len(envelope) != autoUnlockEnvelopeSize {
		return ErrInvalidAutoUnlockWrap
	}
	return writeAutoUnlockFileCreateOnly(path, envelope, "wrapper")
}

func replaceAutoUnlockEnvelope(path string, envelope []byte) (resultErr error) {
	if len(envelope) != autoUnlockEnvelopeSize {
		return ErrInvalidAutoUnlockWrap
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return ErrInvalidAutoUnlockWrap
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect auto unlock wrapper before replacement: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create auto unlock directory: %w", err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return fmt.Errorf("protect auto unlock directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("protect auto unlock directory: %w", err)
		}
	}
	temporary, err := os.CreateTemp(dir, ".qterm-auto-unlock-replace-*")
	if err != nil {
		return fmt.Errorf("create temporary auto unlock wrapper: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := protectLocalPath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("protect temporary auto unlock wrapper: %w", err)
	}
	defer func() {
		if temporary != nil {
			if closeErr := temporary.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("close temporary auto unlock wrapper: %w", closeErr))
			}
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary auto unlock wrapper: %w", removeErr))
		}
	}()
	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			return fmt.Errorf("protect temporary auto unlock wrapper: %w", err)
		}
	}
	written, err := temporary.Write(envelope)
	if err != nil {
		return fmt.Errorf("write temporary auto unlock wrapper: %w", err)
	}
	if written != len(envelope) {
		return io.ErrShortWrite
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary auto unlock wrapper: %w", err)
	}
	if err := temporary.Close(); err != nil {
		temporary = nil
		return fmt.Errorf("close temporary auto unlock wrapper: %w", err)
	}
	temporary = nil
	if err := replaceFileAtomically(temporaryPath, path); err != nil {
		return fmt.Errorf("commit auto unlock wrapper: %w", err)
	}
	if err := syncPasswordKeyDirectory(dir); err != nil {
		return err
	}
	if err := protectLocalPath(path, false); err != nil {
		return fmt.Errorf("protect auto unlock wrapper: %w", err)
	}
	return nil
}

func writeAutoUnlockFileCreateOnly(path string, value []byte, name string) (resultErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create auto unlock directory: %w", err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return fmt.Errorf("protect auto unlock directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("protect auto unlock directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return ErrMasterKeyExists
	}
	if err != nil {
		return fmt.Errorf("create auto unlock %s: %w", name, err)
	}
	if err := protectLocalPath(path, false); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("protect auto unlock %s: %w", name, err)
	}
	removeOnFailure := true
	defer func() {
		if closeErr := file.Close(); closeErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("close auto unlock %s: %w", name, closeErr)
		}
		if removeOnFailure && resultErr != nil {
			_ = os.Remove(path)
		}
	}()
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return fmt.Errorf("protect auto unlock %s: %w", name, err)
		}
	}
	written, err := file.Write(value)
	if err != nil {
		return fmt.Errorf("write auto unlock %s: %w", name, err)
	}
	if written != len(value) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync auto unlock %s: %w", name, err)
	}
	removeOnFailure = false
	return nil
}
