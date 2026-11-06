package credential

import (
	"changeme/internal/errno"
	"changeme/internal/vault"
	"context"
	"errors"
)

func (s *CredentialService) UnlockAutomatically(ctx context.Context) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	if s.security != nil {
		return s.UnlockSecurityAutomatically(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if !s.passwordSet || !s.autoUnlock {
		return s.status, nil
	}
	if s.manager != nil {
		return s.status, nil
	}
	if err := s.unlockAutomaticLocked(ctx); err != nil {
		return s.status, err
	}
	return s.status, nil
}

func (s *CredentialService) SetAutoUnlock(
	ctx context.Context,
	input SetAutoUnlockInput,
) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	password, err := validatedMasterPassword(input.Password)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	defer clear(password)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if !s.passwordSet {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	masterKey, err := s.loadMasterKeyWithPasswordLocked(ctx, password)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}
	defer clear(masterKey)
	if !input.Enabled {
		if err := removeAutoUnlockFilesWithContext(ctx, s.autoKeyPath, s.autoWrapPath); err != nil {
			s.refreshPersistedStateLocked()
			if isContextError(err) {
				return s.status, err
			}
			s.status.Reason = "autoUnlockUpdateFailed"
			s.status.MessageKey = "credential.status.autoUnlockUpdateFailed"
			return s.status, err
		}
		s.autoUnlock = false
		s.refreshStatusAfterSecurityMutationLocked()
		return s.status, nil
	}
	if err := s.rebuildAutoUnlockLocked(ctx, masterKey); err != nil {
		s.refreshPersistedStateLocked()
		s.status.Reason = "autoUnlockUpdateFailed"
		s.status.MessageKey = "credential.status.autoUnlockUpdateFailed"
		return s.status, err
	}
	s.autoUnlock = true
	s.refreshStatusAfterSecurityMutationLocked()
	return s.status, nil
}

func (s *CredentialService) unlockAutomaticLocked(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	key, err := vault.LoadAutoUnlockKey(s.autoKeyPath)
	if err != nil {
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, err)
		return nil
	}
	defer clear(key)
	keyStore, err := s.autoKeyStore(s.autoWrapPath, key)
	if err != nil {
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, err)
		return nil
	}
	masterKey, err := keyStore.Load(ctx)
	closeErr := keyStore.Close()
	if err != nil {
		clear(masterKey)
		if isContextError(err) {
			return err
		}
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, err)
		return nil
	}
	defer clear(masterKey)
	if closeErr != nil {
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, closeErr)
		return nil
	}
	store, err := s.openExistingVaultLocked(ctx, masterKey)
	if err != nil {
		if isContextError(err) {
			return err
		}
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, err)
		return nil
	}
	if err := store.VerifyIdentity(ctx); err != nil {
		_ = store.Close()
		if isContextError(err) {
			return err
		}
		s.status = s.statusForUnlock(CredentialVaultModeAutomatic, err)
		return nil
	}
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return nil
}

func (s *CredentialService) rebuildAutoUnlockLocked(ctx context.Context, masterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(masterKey) != vault.MasterKeySize {
		return vault.ErrInvalidKey
	}
	if err := removeAutoUnlockFilesWithContext(ctx, s.autoKeyPath, s.autoWrapPath); err != nil {
		if isContextError(err) {
			return err
		}
		return errno.New(errno.CodeStoreFailed, "credential.auto_old_cleanup", "")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	autoKey, autoKeyCreated, err := vault.LoadOrCreateAutoUnlockKeyWithStatus(s.autoKeyPath)
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "credential.auto_create_failed", "")
	}
	defer clear(autoKey)
	if err := contextError(ctx); err != nil {
		if autoKeyCreated {
			_ = removeCredentialArtifact(s.autoKeyPath)
		}
		return err
	}
	createdKey := autoKeyCreated
	createdWrapper := false
	cleanup := func() {
		if createdWrapper {
			_ = removeCredentialArtifact(s.autoWrapPath)
		}
		if createdKey {
			_ = removeCredentialArtifact(s.autoKeyPath)
		}
	}
	autoStore, err := s.autoKeyStore(s.autoWrapPath, autoKey)
	if err != nil {
		cleanup()
		return errno.New(errno.CodeStoreFailed, "credential.auto_init_failed", "")
	}
	if err := autoStore.Save(ctx, masterKey); err != nil {
		_ = autoStore.Close()
		cleanup()
		return credentialServiceError("启用自动解锁", err)
	}
	createdWrapper = true
	if err := autoStore.Close(); err != nil {
		cleanup()
		return errno.New(errno.CodeStoreFailed, "credential.auto_cleanup", "")
	}
	if err := contextError(ctx); err != nil {
		cleanup()
		return err
	}
	return nil
}
