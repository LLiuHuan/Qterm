package vault

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	masterPasswordWrapMagic          = "QTMW"
	masterPasswordWrapVersion        = byte(1)
	masterPasswordKDFArgon2id        = byte(1)
	masterPasswordCipherAES256GCM    = byte(1)
	masterPasswordTime               = uint32(3)
	masterPasswordMemoryKiB          = uint32(64 * 1024)
	masterPasswordThreads            = uint8(4)
	masterPasswordSaltSize           = 16
	masterPasswordNonceSize          = 12
	masterPasswordTagSize            = 16
	masterPasswordMaxSize            = 4096
	masterPasswordAdditionalData     = "qterm-vault-master-key-wrap"
	masterPasswordWrapVersionOffset  = len(masterPasswordWrapMagic)
	masterPasswordWrapKDFOffset      = masterPasswordWrapVersionOffset + 1
	masterPasswordArgonVersionOffset = masterPasswordWrapKDFOffset + 1
	masterPasswordCipherOffset       = masterPasswordArgonVersionOffset + 1
	masterPasswordTimeOffset         = masterPasswordCipherOffset + 1
	masterPasswordMemoryOffset       = masterPasswordTimeOffset + 4
	masterPasswordThreadsOffset      = masterPasswordMemoryOffset + 4
	masterPasswordSaltSizeOffset     = masterPasswordThreadsOffset + 1
	masterPasswordHeaderSize         = masterPasswordSaltSizeOffset + 1
	masterPasswordWrappedKeySize     = MasterKeySize + masterPasswordTagSize
	masterPasswordEnvelopeSize       = masterPasswordHeaderSize + masterPasswordSaltSize +
		masterPasswordNonceSize + masterPasswordWrappedKeySize
)

var (
	ErrInvalidMasterPassword        = errors.New("invalid vault master password")
	ErrInvalidMasterKeyWrap         = errors.New("invalid wrapped vault master key")
	ErrUnsupportedMasterKeyWrap     = errors.New("unsupported wrapped vault master key")
	ErrMasterPasswordAuthentication = errors.New("vault master password authentication failed")
)

type masterPasswordParameters struct {
	time      uint32
	memoryKiB uint32
	threads   uint8
	saltSize  uint8
}

type masterPasswordEnvelope struct {
	parameters masterPasswordParameters
	header     []byte
	salt       []byte
	nonce      []byte
	sealedKey  []byte
}

type masterPasswordKeyDeriver func(
	password []byte,
	salt []byte,
	time uint32,
	memoryKiB uint32,
	threads uint8,
	keyLength uint32,
) []byte

type PasswordKeyStore struct {
	mu       sync.Mutex
	path     string
	password []byte
	random   io.Reader
	derive   masterPasswordKeyDeriver
	closed   bool
}

var _ MasterKeyStore = (*PasswordKeyStore)(nil)

func NewPasswordKeyStore(path string, password []byte) (*PasswordKeyStore, error) {
	if err := validatePasswordKeyPath(path); err != nil {
		return nil, err
	}
	if err := validateMasterPassword(password); err != nil {
		return nil, err
	}
	return &PasswordKeyStore{
		path:     filepath.Clean(path),
		password: bytes.Clone(password),
		random:   rand.Reader,
		derive:   argon2.IDKey,
	}, nil
}

func ValidatePasswordKeyFile(path string) error {
	envelope, err := readPasswordKeyEnvelope(path)
	if err != nil {
		return err
	}
	defer clear(envelope)
	_, err = parseMasterPasswordEnvelope(envelope)
	return err
}

func (s *PasswordKeyStore) Load(ctx context.Context) ([]byte, error) {
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
	if err := s.readyLocked(); err != nil {
		return nil, err
	}

	envelope, err := readPasswordKeyEnvelope(s.path)
	if err != nil {
		return nil, err
	}
	defer clear(envelope)
	return unwrapMasterKeyWithPassword(ctx, s.password, envelope, s.derive)
}

func (s *PasswordKeyStore) Save(ctx context.Context, masterKey []byte) error {
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
	if err := s.readyLocked(); err != nil {
		return err
	}

	_, err := os.Lstat(s.path)
	switch {
	case err == nil:
		return ErrMasterKeyExists
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return passwordKeyStoreError("inspect wrapped vault master key", err)
	}

	envelope, err := wrapMasterKeyWithPassword(
		ctx,
		s.password,
		masterKey,
		defaultMasterPasswordParameters(),
		s.random,
		s.derive,
	)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return writePasswordKeyEnvelopeCreateOnly(s.path, envelope)
}

func (s *PasswordKeyStore) Replace(ctx context.Context, masterKey []byte) error {
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
	if err := s.readyLocked(); err != nil {
		return err
	}
	info, err := os.Lstat(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrMasterKeyNotFound
		}
		return passwordKeyStoreError("inspect wrapped vault master key", err)
	}
	if !info.Mode().IsRegular() {
		return ErrInvalidMasterKeyWrap
	}
	if err := protectLocalPath(s.path, false); err != nil {
		return passwordKeyStoreError("protect wrapped vault master key", err)
	}
	envelope, err := wrapMasterKeyWithPassword(
		ctx,
		s.password,
		masterKey,
		defaultMasterPasswordParameters(),
		s.random,
		s.derive,
	)
	if err != nil {
		return err
	}
	defer clear(envelope)
	return replacePasswordKeyEnvelope(s.path, envelope)
}

func (s *PasswordKeyStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	clear(s.password)
	s.password = nil
	s.random = nil
	s.derive = nil
	s.closed = true
	return nil
}

func (s *PasswordKeyStore) readyLocked() error {
	if s.closed || s.path == "" || len(s.password) == 0 || s.random == nil || s.derive == nil {
		return ErrMasterKeyUnavailable
	}
	return nil
}

func wrapMasterKeyWithPassword(
	ctx context.Context,
	password []byte,
	masterKey []byte,
	parameters masterPasswordParameters,
	random io.Reader,
	derive masterPasswordKeyDeriver,
) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateMasterPassword(password); err != nil {
		return nil, err
	}
	if len(masterKey) != MasterKeySize {
		return nil, ErrInvalidKey
	}
	if err := validateMasterPasswordParameters(parameters); err != nil {
		return nil, err
	}
	if random == nil || derive == nil {
		return nil, ErrMasterKeyUnavailable
	}

	passwordCopy := bytes.Clone(password)
	defer clear(passwordCopy)
	masterKeyCopy := bytes.Clone(masterKey)
	defer clear(masterKeyCopy)

	salt := make([]byte, parameters.saltSize)
	if _, err := io.ReadFull(random, salt); err != nil {
		return nil, fmt.Errorf("generate vault master-password salt: %w", err)
	}
	nonce := make([]byte, masterPasswordNonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("generate wrapped master-key nonce: %w", err)
	}

	wrappingKey := deriveMasterPasswordKey(passwordCopy, salt, parameters, derive)
	defer clear(wrappingKey)
	if len(wrappingKey) != MasterKeySize {
		return nil, ErrMasterKeyUnavailable
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	aead, err := newMasterPasswordAEAD(wrappingKey)
	if err != nil {
		return nil, err
	}
	header := encodeMasterPasswordHeader(parameters)
	additionalData := encodeMasterPasswordAdditionalData(header, salt)
	sealedKey := aead.Seal(nil, nonce, masterKeyCopy, additionalData)
	defer clear(sealedKey)

	envelope := make([]byte, 0, masterPasswordEnvelopeSize)
	envelope = append(envelope, header...)
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, sealedKey...)
	if err := contextError(ctx); err != nil {
		clear(envelope)
		return nil, err
	}
	return envelope, nil
}

func unwrapMasterKeyWithPassword(
	ctx context.Context,
	password []byte,
	envelope []byte,
	derive masterPasswordKeyDeriver,
) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateMasterPassword(password); err != nil {
		return nil, err
	}
	if derive == nil {
		return nil, ErrMasterKeyUnavailable
	}

	passwordCopy := bytes.Clone(password)
	defer clear(passwordCopy)
	envelopeCopy := bytes.Clone(envelope)
	defer clear(envelopeCopy)
	parsed, err := parseMasterPasswordEnvelope(envelopeCopy)
	if err != nil {
		return nil, err
	}

	wrappingKey := deriveMasterPasswordKey(passwordCopy, parsed.salt, parsed.parameters, derive)
	defer clear(wrappingKey)
	if len(wrappingKey) != MasterKeySize {
		return nil, ErrMasterKeyUnavailable
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	aead, err := newMasterPasswordAEAD(wrappingKey)
	if err != nil {
		return nil, err
	}
	additionalData := encodeMasterPasswordAdditionalData(parsed.header, parsed.salt)
	masterKey, err := aead.Open(nil, parsed.nonce, parsed.sealedKey, additionalData)
	if err != nil {
		return nil, ErrMasterPasswordAuthentication
	}
	if len(masterKey) != MasterKeySize {
		clear(masterKey)
		return nil, ErrInvalidMasterKeyWrap
	}
	if err := contextError(ctx); err != nil {
		clear(masterKey)
		return nil, err
	}
	return masterKey, nil
}

func parseMasterPasswordEnvelope(envelope []byte) (masterPasswordEnvelope, error) {
	if len(envelope) < masterPasswordHeaderSize {
		return masterPasswordEnvelope{}, ErrInvalidMasterKeyWrap
	}
	if string(envelope[:len(masterPasswordWrapMagic)]) != masterPasswordWrapMagic {
		return masterPasswordEnvelope{}, ErrInvalidMasterKeyWrap
	}
	if envelope[masterPasswordWrapVersionOffset] != masterPasswordWrapVersion ||
		envelope[masterPasswordWrapKDFOffset] != masterPasswordKDFArgon2id ||
		envelope[masterPasswordArgonVersionOffset] != byte(argon2.Version) ||
		envelope[masterPasswordCipherOffset] != masterPasswordCipherAES256GCM {
		return masterPasswordEnvelope{}, ErrUnsupportedMasterKeyWrap
	}
	parameters := masterPasswordParameters{
		time:      binary.BigEndian.Uint32(envelope[masterPasswordTimeOffset:masterPasswordMemoryOffset]),
		memoryKiB: binary.BigEndian.Uint32(envelope[masterPasswordMemoryOffset:masterPasswordThreadsOffset]),
		threads:   envelope[masterPasswordThreadsOffset],
		saltSize:  envelope[masterPasswordSaltSizeOffset],
	}
	if err := validateMasterPasswordParameters(parameters); err != nil {
		return masterPasswordEnvelope{}, err
	}
	if len(envelope) != masterPasswordEnvelopeSize {
		return masterPasswordEnvelope{}, ErrInvalidMasterKeyWrap
	}

	saltStart := masterPasswordHeaderSize
	saltEnd := saltStart + int(parameters.saltSize)
	nonceEnd := saltEnd + masterPasswordNonceSize
	return masterPasswordEnvelope{
		parameters: parameters,
		header:     envelope[:masterPasswordHeaderSize],
		salt:       envelope[saltStart:saltEnd],
		nonce:      envelope[saltEnd:nonceEnd],
		sealedKey:  envelope[nonceEnd:],
	}, nil
}

func defaultMasterPasswordParameters() masterPasswordParameters {
	return masterPasswordParameters{
		time:      masterPasswordTime,
		memoryKiB: masterPasswordMemoryKiB,
		threads:   masterPasswordThreads,
		saltSize:  masterPasswordSaltSize,
	}
}

func validateMasterPasswordParameters(parameters masterPasswordParameters) error {

	if parameters != defaultMasterPasswordParameters() {
		return ErrInvalidMasterKeyWrap
	}
	return nil
}

func validateMasterPassword(password []byte) error {
	if len(password) == 0 || len(password) > masterPasswordMaxSize {
		return ErrInvalidMasterPassword
	}
	return nil
}

func validatePasswordKeyPath(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("wrapped master-key path must be absolute")
	}
	cleanPath := filepath.Clean(path)
	dir := filepath.Dir(cleanPath)
	volumeRoot := filepath.Clean(filepath.VolumeName(dir) + string(os.PathSeparator))
	if cleanPath == dir || filepath.Clean(dir) == volumeRoot {
		return errors.New("wrapped master-key path cannot be in a filesystem root")
	}
	return nil
}

func deriveMasterPasswordKey(
	password []byte,
	salt []byte,
	parameters masterPasswordParameters,
	derive masterPasswordKeyDeriver,
) []byte {
	return derive(
		password,
		salt,
		parameters.time,
		parameters.memoryKiB,
		parameters.threads,
		MasterKeySize,
	)
}

func encodeMasterPasswordHeader(parameters masterPasswordParameters) []byte {
	header := make([]byte, masterPasswordHeaderSize)
	copy(header, masterPasswordWrapMagic)
	header[masterPasswordWrapVersionOffset] = masterPasswordWrapVersion
	header[masterPasswordWrapKDFOffset] = masterPasswordKDFArgon2id
	header[masterPasswordArgonVersionOffset] = byte(argon2.Version)
	header[masterPasswordCipherOffset] = masterPasswordCipherAES256GCM
	binary.BigEndian.PutUint32(header[masterPasswordTimeOffset:], parameters.time)
	binary.BigEndian.PutUint32(header[masterPasswordMemoryOffset:], parameters.memoryKiB)
	header[masterPasswordThreadsOffset] = parameters.threads
	header[masterPasswordSaltSizeOffset] = parameters.saltSize
	return header
}

func encodeMasterPasswordAdditionalData(header, salt []byte) []byte {
	additionalData := make([]byte, 0, len(masterPasswordAdditionalData)+len(header)+len(salt))
	additionalData = append(additionalData, masterPasswordAdditionalData...)
	additionalData = append(additionalData, header...)
	additionalData = append(additionalData, salt...)
	return additionalData
}

func newMasterPasswordAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create wrapped master-key block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create wrapped master-key AEAD: %w", err)
	}
	return aead, nil
}

func readPasswordKeyEnvelope(path string) (envelope []byte, resultErr error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, passwordKeyStoreError("inspect wrapped vault master key", err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(masterPasswordEnvelopeSize) {
		return nil, ErrInvalidMasterKeyWrap
	}
	if err := protectLocalPath(path, false); err != nil {
		return nil, passwordKeyStoreError("protect wrapped vault master key", err)
	}

	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, passwordKeyStoreError("open wrapped vault master key", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, passwordKeyStoreError("close wrapped vault master key", err))
		}
		if resultErr != nil {
			clear(envelope)
			envelope = nil
		}
	}()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, passwordKeyStoreError("verify wrapped vault master key", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, ErrMasterKeyDenied
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return nil, passwordKeyStoreError("protect wrapped vault master key", err)
		}
	}
	envelope, err = io.ReadAll(io.LimitReader(file, int64(masterPasswordEnvelopeSize+1)))
	if err != nil {
		return nil, passwordKeyStoreError("read wrapped vault master key", err)
	}
	if len(envelope) != masterPasswordEnvelopeSize {
		return nil, ErrInvalidMasterKeyWrap
	}
	return envelope, nil
}

func writePasswordKeyEnvelopeCreateOnly(path string, envelope []byte) (resultErr error) {
	if len(envelope) != masterPasswordEnvelopeSize {
		return ErrInvalidMasterKeyWrap
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return passwordKeyStoreError("create wrapped master-key directory", err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return passwordKeyStoreError("protect wrapped master-key directory", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return passwordKeyStoreError("protect wrapped master-key directory", err)
		}
	}

	temporary, err := os.CreateTemp(dir, ".qterm-master-key-*")
	if err != nil {
		return passwordKeyStoreError("create temporary wrapped master key", err)
	}
	temporaryPath := temporary.Name()
	if err := protectLocalPath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return passwordKeyStoreError("protect temporary wrapped master key", err)
	}
	defer func() {
		if temporary != nil {
			if err := temporary.Close(); err != nil {
				resultErr = errors.Join(resultErr, passwordKeyStoreError("close temporary wrapped master key", err))
			}
		}
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			resultErr = errors.Join(resultErr, passwordKeyStoreError("remove temporary wrapped master key", err))
		}
	}()
	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			return passwordKeyStoreError("protect temporary wrapped master key", err)
		}
	}
	written, err := io.Copy(temporary, bytes.NewReader(envelope))
	if err != nil {
		return passwordKeyStoreError("write temporary wrapped master key", err)
	}
	if written != int64(len(envelope)) {
		return passwordKeyStoreError("write temporary wrapped master key", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return passwordKeyStoreError("sync temporary wrapped master key", err)
	}
	if err := temporary.Close(); err != nil {
		temporary = nil
		return passwordKeyStoreError("close temporary wrapped master key", err)
	}
	temporary = nil

	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrMasterKeyExists
		}
		return passwordKeyStoreError("commit wrapped vault master key", err)
	}
	if err := syncPasswordKeyDirectory(dir); err != nil {
		return err
	}
	if err := protectLocalPath(path, false); err != nil {
		return passwordKeyStoreError("protect wrapped vault master key", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return passwordKeyStoreError("remove committed temporary master key", err)
	}
	return nil
}

func replacePasswordKeyEnvelope(path string, envelope []byte) (resultErr error) {
	if len(envelope) != masterPasswordEnvelopeSize {
		return ErrInvalidMasterKeyWrap
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return passwordKeyStoreError("create wrapped master-key directory", err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return passwordKeyStoreError("protect wrapped master-key directory", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return passwordKeyStoreError("protect wrapped master-key directory", err)
		}
	}
	temporary, err := os.CreateTemp(dir, ".qterm-master-key-replace-*")
	if err != nil {
		return passwordKeyStoreError("create temporary wrapped master key", err)
	}
	temporaryPath := temporary.Name()
	if err := protectLocalPath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return passwordKeyStoreError("protect temporary wrapped master key", err)
	}
	defer func() {
		if temporary != nil {
			if closeErr := temporary.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, passwordKeyStoreError("close temporary wrapped master key", closeErr))
			}
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			resultErr = errors.Join(resultErr, passwordKeyStoreError("remove temporary wrapped master key", removeErr))
		}
	}()
	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			return passwordKeyStoreError("protect temporary wrapped master key", err)
		}
	}
	written, err := temporary.Write(envelope)
	if err != nil {
		return passwordKeyStoreError("write temporary wrapped master key", err)
	}
	if written != len(envelope) {
		return passwordKeyStoreError("write temporary wrapped master key", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return passwordKeyStoreError("sync temporary wrapped master key", err)
	}
	if err := temporary.Close(); err != nil {
		temporary = nil
		return passwordKeyStoreError("close temporary wrapped master key", err)
	}
	temporary = nil
	if err := replaceFileAtomically(temporaryPath, path); err != nil {
		return passwordKeyStoreError("commit wrapped vault master key", err)
	}
	if err := syncPasswordKeyDirectory(dir); err != nil {
		return err
	}
	if err := protectLocalPath(path, false); err != nil {
		return passwordKeyStoreError("protect wrapped vault master key", err)
	}
	return nil
}

func passwordKeyStoreError(operation string, err error) error {
	return fmt.Errorf("%s: %v: %w", operation, err, ErrMasterKeyUnavailable)
}
