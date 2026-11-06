package credential

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/security"
	"changeme/internal/vault"
	"context"
	"os"
	"sync"
)

const (
	MinimumMasterPasswordLength = 8
	passwordWrapFileName        = "master-key.wrap"
	autoUnlockKeyFileName       = "auto-unlock.key"
	autoUnlockWrapFileName      = "auto-master-key.wrap"
	passwordHintFileName        = "master-password.hint"
)

var (
	ErrCredentialServiceNotReady         = errno.New(errno.CodeSessionClosed, "credential.not_ready", "")
	ErrCredentialVaultLocked             = model.ErrCredentialVaultLocked
	ErrCredentialVaultAlreadyInitialized = errno.New(errno.CodeInvalidArg, "credential.already_initialized", "")
	ErrCredentialPasswordMismatch        = errno.New(errno.CodeInvalidArg, "credential.password_mismatch", "")
	ErrCredentialPasswordTooShort        = errno.New(errno.CodeInvalidArg, "credential.password_short", "")
	ErrCredentialVaultInUse              = errno.New(errno.CodePermissionDenied, "credential.in_use", "")
	ErrCredentialVaultLockUnavailable    = errno.New(errno.CodePermissionDenied, "credential.lock_unavailable", "")
)

type CredentialVaultState string

const (
	CredentialVaultStateSetupRequired CredentialVaultState = "setupRequired"
	CredentialVaultStateLocked        CredentialVaultState = "locked"
	CredentialVaultStateUnlocked      CredentialVaultState = "unlocked"
	CredentialVaultStateRecovery      CredentialVaultState = "recoveryRequired"
	CredentialVaultStateError         CredentialVaultState = "error"
)

type CredentialVaultMode string

const (
	CredentialVaultModeAutomatic      CredentialVaultMode = "automatic"
	CredentialVaultModeMasterPassword CredentialVaultMode = "masterPassword"
)

type CredentialVaultStatus struct {
	State  CredentialVaultState `json:"state"`
	Mode   CredentialVaultMode  `json:"mode"`
	Reason string               `json:"reason"`

	MessageKey          string                  `json:"messageKey"`
	AutoUnlockEnabled   bool                    `json:"autoUnlockEnabled"`
	PasswordConfigured  bool                    `json:"passwordConfigured"`
	Hint                string                  `json:"hint"`
	EncryptionMode      security.EncryptionMode `json:"encryptionMode,omitempty"`
	KeychainAvailable   bool                    `json:"keychainAvailable,omitempty"`
	CustomKeyConfigured bool                    `json:"customKeyConfigured,omitempty"`
	RecoveryAvailable   bool                    `json:"recoveryAvailable,omitempty"`
}

type ChangeEncryptionKeyInput struct {
	CurrentKey   string `json:"currentKey"`
	NewKey       string `json:"newKey"`
	Confirmation string `json:"confirmation"`
}

type SetupMasterPasswordInput struct {
	Password     string `json:"password"`
	Confirmation string `json:"confirmation"`
	Hint         string `json:"hint"`
}

type ChangeMasterPasswordInput struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
	Hint        string `json:"hint"`
}

type SetAutoUnlockInput struct {
	Enabled  bool   `json:"enabled"`
	Password string `json:"password"`
}

type CreateCredentialInput struct {
	Name       string               `json:"name"`
	Kind       model.CredentialKind `json:"kind"`
	Secret     string               `json:"secret"`
	Passphrase string               `json:"passphrase"`
	Username   string               `json:"username,omitempty"`
	PublicKey  string               `json:"publicKey,omitempty"`
}

type RenameCredentialInput struct {
	Name string `json:"name"`
}

type ReplaceCredentialSecretInput struct {
	Kind       model.CredentialKind `json:"kind"`
	Secret     string               `json:"secret"`
	Passphrase string               `json:"passphrase"`
	Username   string               `json:"username,omitempty"`
	PublicKey  string               `json:"publicKey,omitempty"`
}

type closableMasterKeyStore interface {
	vault.MasterKeyStore
	Close() error
}

type replaceableMasterKeyStore interface {
	closableMasterKeyStore
	Replace(context.Context, []byte) error
}

type passwordKeyStoreFactory func(path string, password []byte) (closableMasterKeyStore, error)
type autoKeyStoreFactory func(path string, key []byte) (closableMasterKeyStore, error)

type CredentialService struct {
	mu sync.RWMutex

	configDir         func() (string, error)
	passwordKeyStore  passwordKeyStoreFactory
	autoKeyStore      autoKeyStoreFactory
	vaultPath         string
	passwordKeyPath   string
	autoKeyPath       string
	autoWrapPath      string
	hintPath          string
	store             *vault.Store
	manager           *CredentialManager
	credentialLinks   *sync.RWMutex
	references        func(context.Context, string) (int, error)
	status            CredentialVaultStatus
	started           bool
	autoUnlock        bool
	passwordSet       bool
	hint              string
	lifecycleLockPath string
	lifecycleLock     *vault.LifecycleLock
	security          *security.Manager
	localKeyMu        sync.Mutex
	localKeys         map[string]string
}

func AttachReferenceCounter(s *CredentialService, links any, references func(context.Context, string) (int, error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.credentialLinks != nil || s.references != nil {
		return
	}
	s.credentialLinks = rwMutexLink(links)
	s.references = references
}

func rwMutexLink(value any) *sync.RWMutex {
	links, _ := value.(*sync.RWMutex)
	return links
}

func NewCredentialService() *CredentialService {
	return newCredentialService(
		os.UserConfigDir,
		func(path string, password []byte) (closableMasterKeyStore, error) {
			return vault.NewPasswordKeyStore(path, password)
		},
		func(path string, key []byte) (closableMasterKeyStore, error) {
			return vault.NewAutoUnlockStore(path, key)
		},
	)
}

func NewCredentialServiceWithSecurity(sec *security.Manager) *CredentialService {
	s := newCredentialService(
		os.UserConfigDir,
		func(path string, password []byte) (closableMasterKeyStore, error) {
			return vault.NewPasswordKeyStore(path, password)
		},
		func(path string, key []byte) (closableMasterKeyStore, error) {
			return vault.NewAutoUnlockStore(path, key)
		},
	)
	s.security = sec
	return s
}

func newCredentialService(
	configDir func() (string, error),
	passwordKeyStore passwordKeyStoreFactory,
	autoKeyStore autoKeyStoreFactory,
) *CredentialService {
	return &CredentialService{
		configDir:        configDir,
		passwordKeyStore: passwordKeyStore,
		autoKeyStore:     autoKeyStore,
		status: CredentialVaultStatus{
			State:      CredentialVaultStateLocked,
			MessageKey: "credential.status.notStarted",
		},
		localKeys: make(map[string]string),
	}
}
