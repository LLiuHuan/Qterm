package credential

import (
	"changeme/internal/vault"
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

func (s *CredentialService) Start(ctx context.Context) error {
	if s == nil || s.configDir == nil || s.passwordKeyStore == nil || s.autoKeyStore == nil {
		return ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	dir, err := s.configDir()
	if err != nil {
		return fmt.Errorf("resolve credential config directory: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("credential service is already started")
	}
	root := filepath.Join(dir, "Qterm")
	s.vaultPath = filepath.Join(root, "vault.db")
	s.passwordKeyPath = filepath.Join(root, "security", "master-key.wrap")
	s.autoKeyPath = filepath.Join(root, "security", "auto-unlock.key")
	s.autoWrapPath = filepath.Join(root, "security", "auto-master-key.wrap")
	s.hintPath = filepath.Join(root, "security", "master-password.hint")
	lifecycleLockPath := filepath.Join(root, "security", "lifecycle.lock")
	lifecycleLock, err := vault.AcquireLifecycleLock(lifecycleLockPath)
	if err != nil {
		if errors.Is(err, vault.ErrLifecycleLockHeld) {
			return ErrCredentialVaultInUse
		}
		return ErrCredentialVaultLockUnavailable
	}
	s.lifecycleLockPath = lifecycleLockPath
	startupComplete := false
	defer func() {
		if !startupComplete {

			_ = s.closeVaultLocked()
			_ = lifecycleLock.Close()
			s.lifecycleLock = nil
			s.lifecycleLockPath = ""
			s.started = false
			s.vaultPath = ""
			s.passwordKeyPath = ""
			s.autoKeyPath = ""
			s.autoWrapPath = ""
			s.hintPath = ""
			s.autoUnlock = false
			s.passwordSet = false
			s.hint = ""
			s.status = CredentialVaultStatus{
				State:      CredentialVaultStateLocked,
				MessageKey: "credential.status.notStarted",
			}
		}
	}()
	if s.security != nil {
		s.lifecycleLock = lifecycleLock
		s.started = true
		if err := s.startWithSecurityLocked(ctx); err != nil {
			return err
		}
		startupComplete = true
		return nil
	}
	s.passwordSet, err = vaultFileExists(s.passwordKeyPath)
	if err != nil {
		return fmt.Errorf("inspect vault password configuration: %w", err)
	}
	hintUnavailable := false
	s.hint, err = vault.LoadPasswordHint(s.hintPath)
	if err != nil {

		s.hint = ""
		hintUnavailable = true
	}
	autoKeyExists, err := vaultRegularFileExists(s.autoKeyPath)
	if err != nil {
		return fmt.Errorf("inspect automatic unlock key: %w", err)
	}
	autoWrapExists, err := vaultRegularFileExists(s.autoWrapPath)
	if err != nil {
		return fmt.Errorf("inspect automatic unlock wrapper: %w", err)
	}

	autoKeyArtifactExists := autoKeyExists
	if !autoKeyArtifactExists {
		autoKeyArtifactExists, err = vaultFileExists(s.autoKeyPath)
		if err != nil {
			return fmt.Errorf("inspect automatic unlock key artifact: %w", err)
		}
	}
	autoWrapArtifactExists := autoWrapExists
	if !autoWrapArtifactExists {
		autoWrapArtifactExists, err = vaultFileExists(s.autoWrapPath)
		if err != nil {
			return fmt.Errorf("inspect automatic unlock wrapper artifact: %w", err)
		}
	}
	s.autoUnlock = autoKeyExists && autoWrapExists
	hintExists, err := vaultFileExists(s.hintPath)
	if err != nil {

		hintExists = true
		hintUnavailable = true
	}
	databaseExists, err := vault.VaultDatabaseExists(s.vaultPath)
	if err != nil {
		return fmt.Errorf("inspect vault database file: %w", err)
	}
	if s.passwordSet {
		vaultExists, inspectErr := vault.VaultExists(s.vaultPath)
		if inspectErr != nil {
			return fmt.Errorf("inspect vault data: %w", inspectErr)
		}
		if !vaultExists || !databaseExists {
			s.autoUnlock = false
			s.status = CredentialVaultStatus{
				State:              CredentialVaultStateRecovery,
				AutoUnlockEnabled:  false,
				PasswordConfigured: true,
				Hint:               s.hint,
				Reason:             "recoveryRequired",
				MessageKey:         "credential.status.databaseMissing",
			}
			s.lifecycleLock = lifecycleLock
			s.started = true
			startupComplete = true
			return nil
		}
		if (autoKeyArtifactExists && !autoKeyExists) || (autoWrapArtifactExists && !autoWrapExists) {
			s.autoUnlock = false
			s.status = s.statusForStateLocked(
				CredentialVaultStateError,
				"autoUnlockArtifactInvalid",
				"credential.status.autoUnlockArtifactInvalid",
			)
			s.lifecycleLock = lifecycleLock
			s.started = true
			startupComplete = true
			return nil
		}
		if wrapErr := vault.ValidatePasswordKeyFile(s.passwordKeyPath); wrapErr != nil {
			s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, wrapErr)
			s.lifecycleLock = lifecycleLock
			s.started = true
			startupComplete = true
			return nil
		}
		if hintUnavailable {
			s.status = s.statusForStateLocked(
				CredentialVaultStateError,
				"passwordHintUnavailable",
				"credential.status.passwordHintUnavailable",
			)
			s.lifecycleLock = lifecycleLock
			s.started = true
			startupComplete = true
			return nil
		}
	}
	if !s.passwordSet {
		vaultExists, inspectErr := vault.VaultExists(s.vaultPath)
		if inspectErr != nil {
			return fmt.Errorf("inspect vault data: %w", inspectErr)
		}
		if vaultExists || autoKeyArtifactExists || autoWrapArtifactExists || hintExists || hintUnavailable {
			s.autoUnlock = false
			s.status = CredentialVaultStatus{
				State:              CredentialVaultStateRecovery,
				AutoUnlockEnabled:  false,
				PasswordConfigured: false,
				Hint:               s.hint,
				Reason:             "recoveryRequired",
				MessageKey:         "credential.status.initializationIncomplete",
			}
		} else {
			s.status = s.statusForStateLocked(CredentialVaultStateSetupRequired, "setupRequired", "credential.status.setupRequired")
		}
		s.lifecycleLock = lifecycleLock
		s.started = true
		startupComplete = true
		return nil
	}
	s.lifecycleLock = lifecycleLock
	s.started = true
	if s.autoUnlock {
		if err := s.unlockAutomaticLocked(ctx); err != nil {
			return err
		}
	} else {
		s.status = s.statusForStateLocked(CredentialVaultStateLocked, "locked", "credential.status.passwordRequired")
	}
	startupComplete = true
	return nil
}

func (s *CredentialService) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.closeVaultLocked()
	if s.security != nil {
		err = errors.Join(err, s.security.Close())
	}
	if s.lifecycleLock != nil {
		err = errors.Join(err, s.lifecycleLock.Close())
		s.lifecycleLock = nil
	}
	s.lifecycleLockPath = ""
	s.started = false
	s.vaultPath = ""
	s.passwordKeyPath = ""
	s.autoKeyPath = ""
	s.autoWrapPath = ""
	s.hintPath = ""
	s.autoUnlock = false
	s.passwordSet = false
	s.hint = ""
	s.localKeyMu.Lock()
	clear(s.localKeys)
	s.localKeyMu.Unlock()
	s.status = CredentialVaultStatus{
		State:      CredentialVaultStateLocked,
		MessageKey: "credential.status.notStarted",
	}
	return err
}

func (s *CredentialService) installVaultLocked(
	store *vault.Store,
	mode CredentialVaultMode,
) {
	repository := vault.NewCredentialRepository(store)
	s.store = store
	s.manager = NewCredentialManager(repository)
	status := CredentialVaultStatus{
		State:              CredentialVaultStateUnlocked,
		Mode:               mode,
		MessageKey:         "credential.status.automaticProtection",
		AutoUnlockEnabled:  s.autoUnlock,
		PasswordConfigured: s.passwordSet,
		Hint:               s.hint,
	}
	if s.security != nil {
		securityStatus := s.security.Status()
		status.EncryptionMode = securityStatus.Mode
		status.KeychainAvailable = securityStatus.KeychainAvailable
		status.CustomKeyConfigured = securityStatus.CustomConfigured
		status.RecoveryAvailable = securityStatus.RecoveryAvailable
	}
	s.status = status
}

func (s *CredentialService) closeVaultLocked() error {
	s.manager = nil
	var result error
	if s.store != nil {
		result = errors.Join(result, s.store.Close())
		s.store = nil
	}
	return result
}
