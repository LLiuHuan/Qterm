package credential

import (
	"changeme/internal/errno"
	"changeme/internal/vault"
	"context"
	"errors"
	"fmt"
)

func (s *CredentialService) SetupMasterPassword(
	ctx context.Context,
	input SetupMasterPasswordInput,
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
	confirmation := []byte(input.Confirmation)
	defer clear(confirmation)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if s.passwordSet {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	if s.status.State != CredentialVaultStateSetupRequired {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	if !bytesEqual(password, confirmation) {
		return s.status, ErrCredentialPasswordMismatch
	}
	hint, err := vault.NormalizePasswordHint(input.Hint)
	if err != nil {
		return s.status, err
	}
	masterKey, err := vault.GenerateMasterKey()
	if err != nil {
		return s.status, fmt.Errorf("生成保险库主密钥失败: %w", err)
	}
	defer clear(masterKey)
	if exists, err := vault.VaultExists(s.vaultPath); err != nil {
		return s.status, credentialServiceError("检查凭据保险库", err)
	} else if exists {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	if err := ensureCredentialVaultArtifactsAbsent(s.autoKeyPath, s.autoWrapPath, s.hintPath); err != nil {
		return s.status, err
	}
	createdPasswordWrapper := false
	createdAutoKey := false
	createdAutoWrapper := false
	createdHint := false
	cleanup := func() {
		_ = removeCredentialArtifact(s.vaultPath)
		_ = removeCredentialArtifact(s.vaultPath + "-wal")
		_ = removeCredentialArtifact(s.vaultPath + "-shm")
		_ = removeCredentialArtifact(s.vaultPath + "-journal")
		if createdHint {
			_ = removeCredentialArtifact(s.hintPath)
		}
		if createdAutoWrapper {
			_ = removeCredentialArtifact(s.autoWrapPath)
		}
		if createdAutoKey {
			_ = removeCredentialArtifact(s.autoKeyPath)
		}
		if createdPasswordWrapper {
			_ = removeCredentialArtifact(s.passwordKeyPath)
		}
	}
	passwordStore, err := s.passwordKeyStore(s.passwordKeyPath, password)
	if err != nil {
		return s.status, credentialServiceError("初始化主密码", err)
	}
	if err := passwordStore.Save(ctx, masterKey); err != nil {
		_ = passwordStore.Close()
		if !errors.Is(err, vault.ErrMasterKeyExists) && artifactExists(s.passwordKeyPath) {
			_ = removeCredentialArtifact(s.passwordKeyPath)
		}
		return s.status, credentialServiceError("保存主密码包装", err)
	}
	createdPasswordWrapper = true
	if err := passwordStore.Close(); err != nil {
		cleanup()
		return s.status, errno.New(errno.CodeStoreFailed, "credential.password_cleanup", "")
	}
	if err := contextError(ctx); err != nil {
		cleanup()
		return s.status, err
	}
	autoKey, autoKeyCreated, err := vault.LoadOrCreateAutoUnlockKeyWithStatus(s.autoKeyPath)
	if err != nil {
		cleanup()
		return s.status, fmt.Errorf("创建自动解锁密钥失败: %w", err)
	}
	createdAutoKey = autoKeyCreated
	defer clear(autoKey)
	autoStore, err := s.autoKeyStore(s.autoWrapPath, autoKey)
	if err != nil {
		cleanup()
		return s.status, fmt.Errorf("初始化自动解锁包装失败: %w", err)
	}
	if err := autoStore.Save(ctx, masterKey); err != nil {
		_ = autoStore.Close()
		if !errors.Is(err, vault.ErrMasterKeyExists) && artifactExists(s.autoWrapPath) {
			createdAutoWrapper = true
		}
		cleanup()
		return s.status, credentialServiceError("保存自动解锁包装", err)
	}
	createdAutoWrapper = true
	if err := autoStore.Close(); err != nil {
		cleanup()
		return s.status, errno.New(errno.CodeStoreFailed, "credential.auto_key_cleanup", "")
	}
	if hint != "" {
		createdHint = true
	}
	if err := savePasswordHintWithContext(ctx, s.hintPath, hint); err != nil {
		cleanup()
		if isContextError(err) {
			return s.status, err
		}
		return s.status, fmt.Errorf("保存主密码提示失败: %w", err)
	}
	store, err := vault.OpenWithMasterKey(ctx, s.vaultPath, masterKey)
	if err != nil {
		cleanup()
		return s.status, fmt.Errorf("打开凭据保险库失败: %w", err)
	}
	if err := store.InitializeIdentity(ctx); err != nil {
		_ = store.Close()
		cleanup()
		return s.status, errno.New(errno.CodeStoreFailed, "credential.identity_failed", "")
	}
	s.passwordSet = true
	s.autoUnlock = true
	s.hint = hint
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return s.status, nil
}

func (s *CredentialService) UnlockWithPassword(
	ctx context.Context,
	password string,
) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	passwordBytes, err := validatedMasterPassword(password)
	if err != nil {
		return CredentialVaultStatus{State: CredentialVaultStateLocked, Mode: CredentialVaultModeMasterPassword}, err
	}
	defer clear(passwordBytes)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if !s.passwordSet {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	if s.manager != nil {
		return s.status, nil
	}
	keyStore, err := s.passwordKeyStore(s.passwordKeyPath, passwordBytes)
	if err != nil {
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}
	masterKey, err := keyStore.Load(ctx)
	closeErr := keyStore.Close()
	if err != nil {
		clear(masterKey)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		if closeErr != nil {
			return s.status, errno.New(errno.CodeStoreFailed, "credential.password_cleanup", "")
		}
		return s.status, nil
	}
	defer clear(masterKey)
	if closeErr != nil {
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, closeErr)
		return s.status, nil
	}
	store, err := s.openExistingVaultLocked(ctx, masterKey)
	if err != nil {
		if isContextError(err) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}
	if err := store.VerifyIdentity(ctx); err != nil {
		_ = store.Close()
		if isContextError(err) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}
	s.installVaultLocked(store, CredentialVaultModeMasterPassword)
	return s.status, nil
}

func (s *CredentialService) ChangeMasterPassword(
	ctx context.Context,
	input ChangeMasterPasswordInput,
) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	oldPassword, err := validatedMasterPassword(input.OldPassword)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	defer clear(oldPassword)
	newPassword, err := validatedMasterPassword(input.NewPassword)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	defer clear(newPassword)
	hint, err := vault.NormalizePasswordHint(input.Hint)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if !s.passwordSet {
		return s.status, ErrCredentialVaultAlreadyInitialized
	}
	oldStore, err := s.passwordKeyStore(s.passwordKeyPath, oldPassword)
	if err != nil {
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}
	masterKey, err := oldStore.Load(ctx)
	closeErr := oldStore.Close()
	if err != nil {
		clear(masterKey)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		if closeErr != nil {
			return s.status, errno.New(errno.CodeStoreFailed, "credential.password_cleanup", "")
		}
		return s.status, nil
	}
	defer clear(masterKey)
	if closeErr != nil {
		return s.status, errno.New(errno.CodeStoreFailed, "credential.password_cleanup", "")
	}
	if err := s.verifyMasterKeyLocked(ctx, masterKey); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return s.status, err
		}
		s.status = s.statusForUnlock(CredentialVaultModeMasterPassword, err)
		return s.status, nil
	}

	previousHint := s.hint
	if err := savePasswordHintWithContext(ctx, s.hintPath, hint); err != nil {
		if isContextError(err) {
			return s.status, err
		}
		return s.status, errno.New(errno.CodeStoreFailed, "credential.hint_failed", "")
	}
	hintPending := true
	defer func() {
		if !hintPending {
			return
		}
		if restoreErr := vault.SavePasswordHint(s.hintPath, previousHint); restoreErr != nil {

			s.refreshPersistedStateLocked()
		}
	}()

	newStore, err := s.passwordKeyStore(s.passwordKeyPath, newPassword)
	if err != nil {
		return s.status, credentialServiceError("初始化新主密码", err)
	}
	passwordStore, ok := newStore.(replaceableMasterKeyStore)
	if !ok {
		_ = newStore.Close()
		return s.status, errno.New(errno.CodePermissionDenied, "credential.password_change_unsupported", "")
	}
	if err := passwordStore.Replace(ctx, masterKey); err != nil {
		_ = passwordStore.Close()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return s.status, err
		}
		if s.passwordWrapperContainsLocked(ctx, newPassword, masterKey) {
			hintPending = false
			s.hint = hint
			s.status.Hint = hint
			s.status.PasswordConfigured = true
			s.status.AutoUnlockEnabled = s.autoUnlock
			s.status.Reason = "passwordChangeDurabilityWarning"
			s.status.MessageKey = "credential.status.passwordChangeDurabilityWarning"
			return s.status, nil
		}
		return s.status, credentialServiceError("修改主密码", err)
	}
	hintPending = false
	s.hint = hint
	if err := passwordStore.Close(); err != nil {
		s.status.Reason = "passwordChangeCleanupFailed"
		s.status.MessageKey = "credential.status.passwordChangeCleanupFailed"
		return s.status, nil
	}
	s.refreshStatusAfterSecurityMutationLocked()
	return s.status, nil
}

func (s *CredentialService) LockCredentialVault() error {
	if s == nil {
		return ErrCredentialServiceNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return ErrCredentialServiceNotReady
	}
	if s.manager == nil && s.store == nil {
		if s.status.State == CredentialVaultStateSetupRequired || s.status.State == CredentialVaultStateRecovery {
			return nil
		}
		s.status = s.statusForStateLocked(CredentialVaultStateLocked, "locked", "credential.status.locked")
		return nil
	}
	closeErr := s.closeVaultLocked()
	if s.security != nil {
		s.security.Lock()
	}
	s.status = s.statusForStateLocked(CredentialVaultStateLocked, "locked", "credential.status.locked")
	if s.security != nil {
		securityStatus := s.security.Status()
		s.status = securityStatusToCredentialStatus(securityStatus)
		s.status.State = CredentialVaultStateLocked
		s.status.Reason = "locked"
		s.status.MessageKey = "credential.status.locked"
	}
	if closeErr != nil {
		s.status.Reason = "lockFailed"
		s.status.MessageKey = "credential.status.lockFailed"
		return errno.New(errno.CodeStoreFailed, "credential.lock_failed", "")
	}
	return nil
}
