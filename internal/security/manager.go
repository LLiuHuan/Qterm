package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type EncryptionMode string

const (
	EncryptionModeSystem EncryptionMode = "system"
	EncryptionModeCustom EncryptionMode = "custom"
)

type Status struct {
	Mode              EncryptionMode `json:"mode"`
	Unlocked          bool           `json:"unlocked"`
	KeychainAvailable bool           `json:"keychainAvailable"`
	CustomConfigured  bool           `json:"customKeyConfigured"`
	RecoveryAvailable bool           `json:"recoveryAvailable"`
	Reason            string         `json:"reason"`
}

type Manager struct {
	mu sync.RWMutex

	root        string
	customStore *CustomKeyStore
	keyring     *SystemKeyStore
	keyringOK   bool
	dek         []byte
	status      Status
	started     bool
}

func NewManager(root string) (*Manager, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, ErrInvalidPath
	}
	root = filepath.Clean(root)
	securityDir := filepath.Join(root, "security")
	customStore, err := NewCustomKeyStore(filepath.Join(securityDir, "custom-key.wrap"))
	if err != nil {
		return nil, err
	}
	accountDigest := sha256.Sum256([]byte(root))
	account := "local-data-encryption-key-" + hex.EncodeToString(accountDigest[:8])
	keyring, err := NewSystemKeyStore(DefaultKeyringService, account)
	if err != nil {
		return nil, err
	}
	return &Manager{root: root, customStore: customStore, keyring: keyring, keyringOK: true}, nil
}

func NewManagerWithStores(root string, keyring *SystemKeyStore, customStore *CustomKeyStore) (*Manager, error) {
	if root == "" || !filepath.IsAbs(root) || keyring == nil || customStore == nil {
		return nil, ErrInvalidPath
	}
	return &Manager{root: filepath.Clean(root), keyring: keyring, customStore: customStore, keyringOK: true}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil || m.keyring == nil || m.customStore == nil {
		return ErrKeyringUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("security manager is already started")
	}
	dek, err := m.keyring.Load(ctx)
	switch {
	case err == nil:
		m.keyringOK = true
		m.installDEKLocked(dek)
		clear(dek)
		m.status = m.statusLocked("", true)
	case errors.Is(err, ErrKeyNotFound):
		if _, customErr := os.Stat(filepath.Join(m.root, "security", "custom-key.wrap")); customErr == nil {
			m.status = m.statusLocked("customKeyRequired", false)
		} else {

			dek, err = m.keyring.LoadOrCreate(ctx)
			if err != nil {
				if errors.Is(err, ErrKeyringUnavailable) {
					m.keyringOK = false
				}
				return err
			}
			m.keyringOK = true
			m.installDEKLocked(dek)
			clear(dek)
			m.status = m.statusLocked("", true)
		}
	case errors.Is(err, ErrKeyringUnavailable):
		m.keyringOK = false
		m.status = m.statusLocked("keychainUnavailable", false)
	default:
		m.keyringOK = false
		m.status = m.statusLocked("keychainUnavailable", false)
	}
	m.started = true
	return nil
}

func (m *Manager) Status() Status {
	if m == nil {
		return Status{Reason: "notStarted"}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) DataKey(ctx context.Context) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrKeyringUnavailable
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.started {
		return nil, errors.New("security manager is not started")
	}
	if len(m.dek) != DEKSize {
		return nil, ErrAuthentication
	}
	return append([]byte(nil), m.dek...), nil
}

func (m *Manager) DerivedKeys(ctx context.Context) (DerivedKeys, error) {
	dek, err := m.DataKey(ctx)
	if err != nil {
		return DerivedKeys{}, err
	}
	defer clear(dek)
	return DeriveKeys(dek)
}

func (m *Manager) ChangeCustomKey(ctx context.Context, currentKey, newKey []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateCustomKey(newKey); err != nil {
		return err
	}
	if m == nil {
		return ErrKeyringUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started || len(m.dek) != DEKSize {
		return ErrAuthentication
	}
	if m.status.CustomConfigured {
		if err := m.customStore.Rotate(ctx, currentKey, newKey); err != nil {
			return err
		}
	} else if err := m.customStore.Save(ctx, newKey, m.dek); err != nil {
		return err
	}
	m.status = m.statusLocked("", true)
	return nil
}

func (m *Manager) UnlockCustomKey(ctx context.Context, key []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil {
		return ErrKeyringUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	dek, err := m.customStore.Load(ctx, key)
	if err != nil {
		m.status = m.statusLocked("authenticationFailed", false)
		return err
	}
	m.installDEKLocked(dek)
	clear(dek)
	m.status = m.statusLocked("", true)
	return nil
}

func (m *Manager) PersistSystemKey(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil || m.keyring == nil {
		return ErrKeyringUnavailable
	}
	dek, err := m.DataKey(ctx)
	if err != nil {
		return err
	}
	defer clear(dek)
	if err := m.keyring.Replace(ctx, dek); err != nil {
		m.mu.Lock()
		if errors.Is(err, ErrKeyringUnavailable) {
			m.keyringOK = false
		}
		m.status = m.statusLocked(m.status.Reason, len(m.dek) == DEKSize)
		m.mu.Unlock()
		return err
	}
	m.mu.Lock()
	m.keyringOK = true
	m.status = m.statusLocked(m.status.Reason, len(m.dek) == DEKSize)
	m.mu.Unlock()
	return nil
}

func (m *Manager) UnlockSystem(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil {
		return ErrKeyringUnavailable
	}
	dek, err := m.keyring.Load(ctx)
	if err != nil {
		m.mu.Lock()
		if errors.Is(err, ErrKeyringUnavailable) {
			m.keyringOK = false
		}
		m.mu.Unlock()
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keyringOK = true
	m.installDEKLocked(dek)
	clear(dek)
	m.status = m.statusLocked("", true)
	return nil
}

func (m *Manager) Lock() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.dek)
	m.dek = nil
	m.status = m.statusLocked("locked", false)
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.dek)
	m.dek = nil
	m.started = false
	m.status = Status{Mode: EncryptionModeSystem, Reason: "notStarted"}
	return nil
}

func (m *Manager) installDEKLocked(dek []byte) {
	clear(m.dek)
	m.dek = append(m.dek[:0], dek...)
}

func (m *Manager) statusLocked(reason string, unlocked bool) Status {
	_, customErr := os.Stat(filepath.Join(m.root, "security", "custom-key.wrap"))
	customConfigured := customErr == nil
	mode := EncryptionModeSystem
	if customConfigured {
		mode = EncryptionModeCustom
	}
	return Status{
		Mode:              mode,
		Unlocked:          unlocked,
		KeychainAvailable: m.keyringOK,
		CustomConfigured:  customConfigured,
		RecoveryAvailable: true,
		Reason:            reason,
	}
}

func (m *Manager) ExportRecovery(ctx context.Context, path string, recoveryKey []byte) error {
	dek, err := m.DataKey(ctx)
	if err != nil {
		return err
	}
	defer clear(dek)
	return ExportRecoveryPackage(ctx, path, dek, recoveryKey)
}

func (m *Manager) ImportRecovery(ctx context.Context, path string, recoveryKey []byte) error {
	if m == nil || m.keyring == nil || m.customStore == nil {
		return ErrKeyringUnavailable
	}
	dek, err := ReadRecoveryPackage(ctx, path, recoveryKey)
	if err != nil {
		return err
	}
	defer clear(dek)
	return m.InstallRecoveryDEK(ctx, dek)
}

func (m *Manager) InstallRecoveryDEK(ctx context.Context, dek []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil || m.keyring == nil || m.customStore == nil {
		return ErrKeyringUnavailable
	}
	if !validDEK(dek) {
		return ErrInvalidDEK
	}

	if exists, existsErr := m.customStore.Exists(ctx); existsErr == nil && exists {
		if deleteErr := m.customStore.Delete(ctx); deleteErr != nil {
			return deleteErr
		}
	} else if existsErr != nil && !errors.Is(existsErr, ErrInvalidEnvelope) {
		return existsErr
	} else if existsErr != nil {
		if deleteErr := m.customStore.Delete(ctx); deleteErr != nil {
			return deleteErr
		}
	}
	keyringErr := m.keyring.Replace(ctx, dek)
	if keyringErr != nil && !errors.Is(keyringErr, ErrKeyringUnavailable) {
		return keyringErr
	}
	m.mu.Lock()
	m.keyringOK = keyringErr == nil
	m.installDEKLocked(dek)
	m.status = m.statusLocked("", true)
	m.mu.Unlock()
	return nil
}

func (m *Manager) ClearCustomKey(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if m == nil || m.customStore == nil {
		return ErrKeyringUnavailable
	}
	if err := m.customStore.Delete(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	m.status = m.statusLocked("", len(m.dek) == DEKSize)
	m.mu.Unlock()
	return nil
}

func (m *Manager) Root() string {
	if m == nil {
		return ""
	}
	return m.root
}

func (m *Manager) String() string {
	return fmt.Sprintf("security manager(%s)", m.Root())
}
