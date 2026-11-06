package credential

import (
	"changeme/internal/vault"
	"errors"
)

func (s *CredentialService) CredentialStatus() (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	return s.status, nil
}

func (s *CredentialService) refreshStatusAfterSecurityMutationLocked() {
	if s == nil {
		return
	}
	if s.manager != nil || s.store != nil {
		mode := s.status.Mode
		if mode == "" {
			mode = modeForStatus(s.autoUnlock)
		}
		messageKey := "credential.status.passwordProtection"
		if s.autoUnlock {
			messageKey = "credential.status.automaticProtection"
		}
		s.status = CredentialVaultStatus{
			State:              CredentialVaultStateUnlocked,
			Mode:               mode,
			MessageKey:         messageKey,
			AutoUnlockEnabled:  s.autoUnlock,
			PasswordConfigured: s.passwordSet,
			Hint:               s.hint,
		}
		return
	}
	if hint, err := vault.LoadPasswordHint(s.hintPath); err == nil {
		s.hint = hint
		s.status = s.statusForStateLocked(
			CredentialVaultStateLocked,
			"locked",
			"credential.status.passwordRequired",
		)
		return
	}
	s.hint = ""
	s.status = s.statusForStateLocked(
		CredentialVaultStateError,
		"passwordHintUnavailable",
		"credential.status.passwordHintUnavailable",
	)
}

func (s *CredentialService) statusForStateLocked(state CredentialVaultState, reason, messageKey string) CredentialVaultStatus {
	mode := CredentialVaultMode("")
	if s.passwordSet {
		mode = modeForStatus(s.autoUnlock)
	}
	return CredentialVaultStatus{
		State:              state,
		Mode:               mode,
		Reason:             reason,
		MessageKey:         messageKey,
		AutoUnlockEnabled:  s.autoUnlock,
		PasswordConfigured: s.passwordSet,
		Hint:               s.hint,
	}
}

func (s *CredentialService) persistedModeLocked() CredentialVaultMode {
	if s == nil || !s.passwordSet {
		return ""
	}
	return modeForStatus(s.autoUnlock)
}

func (s *CredentialService) statusForUnlock(mode CredentialVaultMode, err error) CredentialVaultStatus {
	status := credentialUnlockStatus(mode, err)

	if (s.manager != nil || s.store != nil) && s.status.State == CredentialVaultStateUnlocked {
		if status.Reason == "authenticationFailed" {
			status.State = CredentialVaultStateUnlocked
			status.Mode = s.status.Mode
		} else {

			_ = s.closeVaultLocked()
		}
	}
	status.AutoUnlockEnabled = s.autoUnlock
	status.PasswordConfigured = s.passwordSet
	status.Hint = s.hint
	return status
}

func credentialUnlockStatus(mode CredentialVaultMode, err error) CredentialVaultStatus {
	status := CredentialVaultStatus{
		State: CredentialVaultStateLocked,
		Mode:  mode,
	}
	switch {
	case errors.Is(err, vault.ErrRecoveryRequired):
		status.State = CredentialVaultStateRecovery
		status.Reason = "recoveryRequired"
		status.MessageKey = "credential.status.masterKeyMissing"
	case mode == CredentialVaultModeMasterPassword && errors.Is(err, vault.ErrMasterKeyNotFound):
		status.State = CredentialVaultStateRecovery
		status.Reason = "recoveryRequired"
		status.MessageKey = "credential.status.passwordWrapMissing"
	case mode == CredentialVaultModeMasterPassword &&
		(errors.Is(err, vault.ErrInvalidMasterKeyWrap) || errors.Is(err, vault.ErrUnsupportedMasterKeyWrap)):
		status.State = CredentialVaultStateRecovery
		status.Reason = "recoveryRequired"
		status.MessageKey = "credential.status.passwordWrapDamaged"
	case mode == CredentialVaultModeMasterPassword &&
		(errors.Is(err, vault.ErrVaultIdentityMissing) || errors.Is(err, vault.ErrVaultIdentityAuthentication)):
		status.State = CredentialVaultStateRecovery
		status.Reason = "recoveryRequired"
		status.MessageKey = "credential.status.vaultIdentityMismatch"
	case errors.Is(err, vault.ErrMasterPasswordAuthentication):
		status.Reason = "authenticationFailed"
		status.MessageKey = "credential.status.authenticationFailed"
	case errors.Is(err, vault.ErrAutoUnlockAuthentication):
		status.Reason = "autoUnlockFailed"
		status.MessageKey = "credential.status.autoUnlockMismatch"
	case mode == CredentialVaultModeAutomatic && (errors.Is(err, vault.ErrMasterKeyNotFound) ||
		errors.Is(err, vault.ErrInvalidAutoUnlockKey) || errors.Is(err, vault.ErrInvalidAutoUnlockWrap) ||
		errors.Is(err, vault.ErrUnsupportedMasterKeyWrap) ||
		errors.Is(err, vault.ErrVaultIdentityMissing) || errors.Is(err, vault.ErrVaultIdentityAuthentication)):
		status.Reason = "autoUnlockFailed"
		status.MessageKey = "credential.status.autoUnlockFailed"
	case errors.Is(err, vault.ErrMasterKeyDenied):
		status.Reason = "accessDenied"
		status.MessageKey = "credential.status.accessDenied"
	case errors.Is(err, vault.ErrMasterKeyUnavailable):
		status.Reason = "storageUnavailable"
		status.MessageKey = "credential.status.storageUnavailable"
	case errors.Is(err, vault.ErrInvalidMasterPassword):
		status.Reason = "invalidPassword"
		status.MessageKey = "credential.status.invalidPassword"
	default:
		status.State = CredentialVaultStateError
		status.Reason = "unlockFailed"
		status.MessageKey = "credential.status.unlockFailed"
	}
	return status
}

func (s *CredentialService) refreshPersistedStateLocked() {
	if s == nil {
		return
	}
	if exists, err := vaultFileExists(s.passwordKeyPath); err == nil {
		s.passwordSet = exists
	} else {
		s.passwordSet = true
	}
	autoKeyExists, keyErr := vaultRegularFileExists(s.autoKeyPath)
	autoWrapExists, wrapErr := vaultRegularFileExists(s.autoWrapPath)
	if keyErr == nil && wrapErr == nil {
		s.autoUnlock = autoKeyExists && autoWrapExists
	} else {
		s.autoUnlock = false
	}
	if hint, err := vault.LoadPasswordHint(s.hintPath); err == nil {
		s.hint = hint
	} else {
		s.hint = ""
	}
	s.status.AutoUnlockEnabled = s.autoUnlock
	s.status.PasswordConfigured = s.passwordSet
	s.status.Hint = s.hint

	if s.status.State != CredentialVaultStateUnlocked {
		s.status.Mode = s.persistedModeLocked()
	}
}

func modeForStatus(autoUnlock bool) CredentialVaultMode {
	if autoUnlock {
		return CredentialVaultModeAutomatic
	}
	return CredentialVaultModeMasterPassword
}
