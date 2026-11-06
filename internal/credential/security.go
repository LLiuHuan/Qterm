package credential

import (
	"changeme/internal/errno"
	"changeme/internal/security"
	"changeme/internal/vault"
	"context"
	"errors"
)

func (s *CredentialService) startWithSecurityLocked(ctx context.Context) error {
	securityStatus := s.security.Status()
	s.autoUnlock = true
	s.passwordSet = securityStatus.CustomConfigured
	s.status = securityStatusToCredentialStatus(securityStatus)
	if !securityStatus.Unlocked {
		return nil
	}
	store, err := s.openSecurityVaultLocked(ctx)
	if err != nil {
		if errors.Is(err, vault.ErrVaultIdentityMissing) || errors.Is(err, vault.ErrVaultIdentityAuthentication) {

			migrated, migrationErr := s.migrateLegacyVaultLocked(ctx)
			if migrationErr == nil && migrated {
				store, err = s.openSecurityVaultLocked(ctx)
			} else if migrationErr != nil {
				err = migrationErr
			}
		}
		if err != nil {
			s.security.Lock()
			s.status = securityStatusToCredentialStatus(s.security.Status())
			s.status.State = CredentialVaultStateRecovery
			s.status.Reason = "recoveryRequired"
			s.status.MessageKey = "credential.status.vaultIdentityMismatch"
			return nil
		}
	}
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return nil
}

func (s *CredentialService) migrateLegacyVaultLocked(ctx context.Context) (bool, error) {
	legacyKey, err := vault.LoadAutoUnlockKey(s.autoKeyPath)
	if err != nil {
		if errors.Is(err, vault.ErrMasterKeyNotFound) || errors.Is(err, vault.ErrInvalidAutoUnlockKey) {
			return false, nil
		}
		return false, err
	}
	defer clear(legacyKey)
	legacyStore, err := vault.NewAutoUnlockStore(s.autoWrapPath, legacyKey)
	if err != nil {
		return false, err
	}
	legacyMasterKey, err := legacyStore.Load(ctx)
	_ = legacyStore.Close()
	if err != nil {
		if errors.Is(err, vault.ErrMasterKeyNotFound) || errors.Is(err, vault.ErrInvalidAutoUnlockWrap) || errors.Is(err, vault.ErrAutoUnlockAuthentication) {
			return false, nil
		}
		return false, err
	}
	defer clear(legacyMasterKey)
	legacyStoreOpened, err := vault.OpenExistingWithMasterKey(ctx, s.vaultPath, legacyMasterKey)
	if err != nil {
		return false, nil
	}
	if err := legacyStoreOpened.VerifyIdentity(ctx); err != nil {
		_ = legacyStoreOpened.Close()
		return false, nil
	}
	keys, err := s.security.DerivedKeys(ctx)
	if err != nil {
		_ = legacyStoreOpened.Close()
		return false, err
	}
	defer clear(keys.Vault)
	defer clear(keys.Qterm)
	defer clear(keys.Sync)
	if err := legacyStoreOpened.Rekey(ctx, keys.Vault); err != nil {
		_ = legacyStoreOpened.Close()
		return false, err
	}
	if err := legacyStoreOpened.Close(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *CredentialService) openSecurityVaultLocked(ctx context.Context) (*vault.Store, error) {
	keys, err := s.security.DerivedKeys(ctx)
	if err != nil {
		return nil, err
	}
	defer clear(keys.Vault)
	defer clear(keys.Qterm)
	defer clear(keys.Sync)
	exists, err := vault.VaultDatabaseExists(s.vaultPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		store, openErr := vault.OpenWithMasterKey(ctx, s.vaultPath, keys.Vault)
		if openErr != nil {
			return nil, openErr
		}
		if identityErr := store.InitializeIdentity(ctx); identityErr != nil {
			_ = store.Close()
			return nil, identityErr
		}
		return store, nil
	}
	store, err := vault.OpenExistingWithMasterKey(ctx, s.vaultPath, keys.Vault)
	if err != nil {
		return nil, err
	}
	if err := store.VerifyIdentity(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *CredentialService) ChangeEncryptionKey(ctx context.Context, input ChangeEncryptionKeyInput) (CredentialVaultStatus, error) {
	if s == nil || s.security == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	if input.NewKey != input.Confirmation {
		return s.status, ErrCredentialPasswordMismatch
	}
	newKey, err := validatedMasterPassword(input.NewKey)
	if err != nil {
		return s.status, err
	}
	defer clear(newKey)
	currentKey := []byte(input.CurrentKey)
	defer clear(currentKey)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := s.security.ChangeCustomKey(ctx, currentKey, newKey); err != nil {
		if errors.Is(err, security.ErrAuthentication) {
			return s.status, ErrCredentialPasswordMismatch
		}
		return s.status, errno.New(errno.CodeStoreFailed, "credential.encryption_key_change_failed", "")
	}
	s.passwordSet = true
	s.refreshSecurityStatusLocked()
	return s.status, nil
}

func (s *CredentialService) UnlockWithCustomKey(ctx context.Context, key string) (CredentialVaultStatus, error) {
	if s == nil || s.security == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	keyBytes := []byte(key)
	defer clear(keyBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := s.security.UnlockCustomKey(ctx, keyBytes); err != nil {
		s.status = securityStatusToCredentialStatus(s.security.Status())
		s.status.State = CredentialVaultStateLocked
		s.status.Reason = "authenticationFailed"
		s.status.MessageKey = "credential.status.authenticationFailed"
		return s.status, nil
	}
	store, err := s.openSecurityVaultLocked(ctx)
	if err != nil {
		s.security.Lock()
		s.status = securityStatusToCredentialStatus(s.security.Status())
		s.status.State = CredentialVaultStateRecovery
		s.status.Reason = "recoveryRequired"
		s.status.MessageKey = "credential.status.databaseMissing"
		return s.status, nil
	}

	_ = s.security.PersistSystemKey(ctx)
	s.passwordSet = true
	s.autoUnlock = true
	s.installVaultLocked(store, CredentialVaultModeMasterPassword)
	return s.status, nil
}

func (s *CredentialService) UnlockSecurityAutomatically(ctx context.Context) (CredentialVaultStatus, error) {
	if s == nil || s.security == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.security.UnlockSystem(ctx); err != nil {
		s.status = securityStatusToCredentialStatus(s.security.Status())
		s.status.State = CredentialVaultStateLocked
		return s.status, nil
	}
	store, err := s.openSecurityVaultLocked(ctx)
	if err != nil {
		s.security.Lock()
		s.status = securityStatusToCredentialStatus(s.security.Status())
		s.status.State = CredentialVaultStateRecovery
		s.status.Reason = "recoveryRequired"
		s.status.MessageKey = "credential.status.vaultIdentityMismatch"
		return s.status, nil
	}
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return s.status, nil
}

func (s *CredentialService) refreshSecurityStatusLocked() {
	securityStatus := s.security.Status()
	status := securityStatusToCredentialStatus(securityStatus)
	status.State = CredentialVaultStateUnlocked
	status.Mode = CredentialVaultModeAutomatic
	status.AutoUnlockEnabled = true
	status.PasswordConfigured = securityStatus.CustomConfigured
	s.status = status
}

func securityStatusToCredentialStatus(status security.Status) CredentialVaultStatus {
	result := CredentialVaultStatus{
		Mode:                CredentialVaultModeAutomatic,
		Reason:              status.Reason,
		EncryptionMode:      status.Mode,
		KeychainAvailable:   status.KeychainAvailable,
		CustomKeyConfigured: status.CustomConfigured,
		RecoveryAvailable:   status.RecoveryAvailable,
		AutoUnlockEnabled:   true,
		PasswordConfigured:  status.CustomConfigured,
	}
	if status.Unlocked {
		result.State = CredentialVaultStateUnlocked
		result.MessageKey = "credential.status.automaticProtection"
		return result
	}
	result.State = CredentialVaultStateLocked
	if status.Reason == "customKeyRequired" {
		result.MessageKey = "credential.status.customKeyRequired"
	} else if status.Reason == "keychainUnavailable" {
		result.MessageKey = "credential.status.keychainUnavailable"
	} else if status.Reason == "authenticationFailed" {
		result.MessageKey = "credential.status.authenticationFailed"
	} else {
		result.MessageKey = "credential.status.passwordRequired"
	}
	return result
}
