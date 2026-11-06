package credential

import (
	"bytes"
	"changeme/internal/errno"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"changeme/internal/security"
	"changeme/internal/vault"
)

const (
	CredentialVaultResetConfirmationPhrase = "RESET QTERM VAULT"
)

var (
	ErrCredentialVaultResetNotAllowed               = errno.New(errno.CodePermissionDenied, "credential.reset_unavailable", "")
	ErrCredentialVaultResetConfirmation             = errno.New(errno.CodeInvalidArg, "credential.reset_confirmation", "")
	ErrCredentialVaultResetFailed                   = errno.New(errno.CodeStoreFailed, "credential.reset_failed", "")
	ErrCredentialVaultRecoveryNotAllowed            = errno.New(errno.CodePermissionDenied, "credential.import_unavailable", "")
	ErrCredentialVaultRecoveryConflict              = errno.New(errno.CodePermissionDenied, "credential.import_conflict", "")
	ErrCredentialVaultRecoveryPackageAuthentication = errno.New(errno.CodeInvalidArg, "credential.recovery_password", "")
	ErrCredentialVaultRecoveryPackageInvalid        = errno.New(errno.CodeInvalidArg, "credential.recovery_invalid", "")
	ErrCredentialVaultRecoveryFailed                = errno.New(errno.CodeStoreFailed, "credential.recovery_failed", "")
	ErrCredentialVaultExportNotAllowed              = errno.New(errno.CodePermissionDenied, "credential.export_locked", "")
	ErrCredentialVaultExportAuthentication          = errno.New(errno.CodeInvalidArg, "credential.current_password", "")
	ErrCredentialVaultExportPasswordMismatch        = errno.New(errno.CodeInvalidArg, "credential.recovery_password_mismatch", "")
	ErrCredentialVaultExportConflict                = errno.New(errno.CodePermissionDenied, "credential.export_exists", "")
	ErrCredentialVaultExportFailed                  = errno.New(errno.CodeStoreFailed, "credential.export_failed", "")
	ErrCredentialVaultPackagePath                   = errno.New(errno.CodeInvalidArg, "credential.recovery_path", "")
)

type ResetCredentialVaultInput struct {
	Confirmation string `json:"confirmation"`
}

type ExportCredentialVaultKeyInput struct {
	Path            string `json:"path"`
	CurrentPassword string `json:"currentPassword"`
	PackagePassword string `json:"packagePassword"`
	Confirmation    string `json:"confirmation"`
}

type ImportCredentialVaultKeyInput struct {
	Path            string `json:"path"`
	PackagePassword string `json:"packagePassword"`
	MasterPassword  string `json:"masterPassword"`
	Confirmation    string `json:"confirmation"`
	Hint            string `json:"hint"`
}

func (s *CredentialService) ResetCredentialVault(
	ctx context.Context,
	input ResetCredentialVaultInput,
) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if strings.TrimSpace(input.Confirmation) != CredentialVaultResetConfirmationPhrase {
		return s.status, ErrCredentialVaultResetConfirmation
	}
	if s.manager != nil || s.store != nil || !credentialVaultRecoveryState(s.status.State) {
		return s.status, ErrCredentialVaultResetNotAllowed
	}
	if err := contextError(ctx); err != nil {
		return s.status, err
	}
	if s.security != nil {
		return s.resetSecurityVaultLocked(ctx)
	}

	if err := removeCredentialVaultArtifactsContext(ctx,
		s.vaultPath,
		s.vaultPath+"-wal",
		s.vaultPath+"-shm",
		s.vaultPath+"-journal",
		s.passwordKeyPath,
		s.autoKeyPath,
		s.autoWrapPath,
		s.hintPath,
	); err != nil {
		if isContextError(err) {
			s.refreshPersistedStateLocked()
			s.status = s.statusForStateLocked(CredentialVaultStateRecovery, "resetInterrupted", "credential.status.resetInterrupted")
			return s.status, err
		}
		passwordExists, passwordErr := vaultFileExists(s.passwordKeyPath)
		autoKeyExists, autoKeyErr := vaultRegularFileExists(s.autoKeyPath)
		autoWrapExists, autoWrapErr := vaultRegularFileExists(s.autoWrapPath)
		if passwordErr == nil {
			s.passwordSet = passwordExists
		}
		if autoKeyErr == nil && autoWrapErr == nil {
			s.autoUnlock = autoKeyExists && autoWrapExists
		} else {
			s.autoUnlock = false
		}
		if hint, hintErr := vault.LoadPasswordHint(s.hintPath); hintErr == nil {
			s.hint = hint
		} else {
			s.hint = ""
		}
		s.status = s.statusForStateLocked(CredentialVaultStateRecovery, "resetFailed", "credential.status.resetFailed")
		return s.status, ErrCredentialVaultResetFailed
	}

	s.passwordSet = false
	s.autoUnlock = false
	s.hint = ""
	s.status = s.statusForStateLocked(CredentialVaultStateSetupRequired, "setupRequired", "credential.status.setupRequired")
	return s.status, nil
}

func (s *CredentialService) resetSecurityVaultLocked(ctx context.Context) (CredentialVaultStatus, error) {
	if s.security == nil || s.manager != nil || s.store != nil || !credentialVaultRecoveryState(s.status.State) {
		return s.status, ErrCredentialVaultResetNotAllowed
	}
	if err := s.security.UnlockSystem(ctx); err != nil {
		return s.status, ErrCredentialVaultResetFailed
	}
	if err := s.security.ClearCustomKey(ctx); err != nil {
		return s.status, ErrCredentialVaultResetFailed
	}
	if err := removeCredentialVaultArtifactsContext(ctx,
		s.vaultPath,
		s.vaultPath+"-wal",
		s.vaultPath+"-shm",
		s.vaultPath+"-journal",
		s.passwordKeyPath,
		s.autoKeyPath,
		s.autoWrapPath,
		s.hintPath,
	); err != nil {
		s.security.Lock()
		return s.status, ErrCredentialVaultResetFailed
	}
	store, err := s.openSecurityVaultLocked(ctx)
	if err != nil {
		s.security.Lock()
		return s.status, ErrCredentialVaultResetFailed
	}
	s.passwordSet = false
	s.autoUnlock = true
	s.hint = ""
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return s.status, nil
}

func (s *CredentialService) ExportCredentialVaultKey(
	ctx context.Context,
	input ExportCredentialVaultKeyInput,
) error {
	if s == nil {
		return ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	packagePassword, err := validatedMasterPassword(input.PackagePassword)
	if err != nil {
		return err
	}
	defer clear(packagePassword)
	confirmation := []byte(input.Confirmation)
	defer clear(confirmation)
	if !bytesEqual(packagePassword, confirmation) {
		return ErrCredentialVaultExportPasswordMismatch
	}
	if s.security != nil {
		return s.exportSecurityRecovery(ctx, input, packagePassword)
	}
	currentPassword, err := validatedMasterPassword(input.CurrentPassword)
	if err != nil {
		return err
	}
	defer clear(currentPassword)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return ErrCredentialServiceNotReady
	}
	if s.manager == nil || s.store == nil || s.status.State != CredentialVaultStateUnlocked || !s.passwordSet {
		return ErrCredentialVaultExportNotAllowed
	}
	if err := validateCredentialVaultPackagePath(input.Path, s.vaultPackageForbiddenPaths()...); err != nil {
		return err
	}

	masterKey, err := s.loadMasterKeyWithPasswordLocked(ctx, currentPassword)
	if err != nil {
		if isContextError(err) {
			return err
		}
		if errors.Is(err, vault.ErrMasterPasswordAuthentication) {
			return ErrCredentialVaultExportAuthentication
		}
		return ErrCredentialVaultExportFailed
	}
	defer clear(masterKey)
	source := &snapshotMasterKeyStore{key: masterKey}
	if err := vault.ExportPortableMasterKey(ctx, source, input.Path, packagePassword); err != nil {
		if errors.Is(err, vault.ErrMasterKeyExists) {
			return ErrCredentialVaultExportConflict
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrCredentialVaultExportFailed
	}
	return nil
}

func (s *CredentialService) ImportCredentialVaultKey(
	ctx context.Context,
	input ImportCredentialVaultKeyInput,
) (CredentialVaultStatus, error) {
	if s == nil {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return CredentialVaultStatus{}, err
	}
	packagePassword, err := validatedMasterPassword(input.PackagePassword)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	defer clear(packagePassword)
	if s.security != nil {
		return s.importSecurityRecovery(ctx, input, packagePassword)
	}
	masterPassword, err := validatedMasterPassword(input.MasterPassword)
	if err != nil {
		return CredentialVaultStatus{}, err
	}
	defer clear(masterPassword)
	confirmation := []byte(input.Confirmation)
	defer clear(confirmation)
	if !bytesEqual(masterPassword, confirmation) {
		return CredentialVaultStatus{}, ErrCredentialPasswordMismatch
	}
	hint, err := vault.NormalizePasswordHint(input.Hint)
	if err != nil {
		return CredentialVaultStatus{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if s.manager != nil || s.store != nil || !credentialVaultRecoveryState(s.status.State) {
		return s.status, ErrCredentialVaultRecoveryNotAllowed
	}
	if err := validateCredentialVaultPackagePath(input.Path, s.vaultPackageForbiddenPaths()...); err != nil {
		return s.status, err
	}
	passwordWrapperExists, err := vaultFileExists(s.passwordKeyPath)
	if err != nil {
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	vaultExists, err := vault.VaultDatabaseExists(s.vaultPath)
	if err != nil || !vaultExists {
		return s.status, ErrCredentialVaultRecoveryNotAllowed
	}

	imported := &snapshotMasterKeyStore{loadErr: vault.ErrMasterKeyNotFound}
	defer imported.clear()
	if err := vault.ImportPortableMasterKey(ctx, imported, input.Path, packagePassword); err != nil {
		return s.status, mapCredentialVaultImportError(err)
	}
	masterKey, err := imported.Load(ctx)
	if err != nil {
		if isContextError(err) {
			return s.status, err
		}
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	defer clear(masterKey)

	store, err := vault.OpenExistingWithMasterKey(ctx, s.vaultPath, masterKey)
	if err != nil {
		return s.status, mapCredentialVaultOpenError(err)
	}
	keepStore := false
	defer func() {
		if !keepStore {
			_ = store.Close()
		}
	}()
	if err := verifyImportedVault(ctx, store); err != nil {
		if isContextError(err) {
			return s.status, err
		}
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	artifactsMutated := true
	defer func() {
		if artifactsMutated {
			s.refreshPersistedStateLocked()
		}
	}()
	if err := removeCredentialVaultArtifactsContext(ctx, s.autoKeyPath, s.autoWrapPath, s.hintPath); err != nil {
		if isContextError(err) {
			return s.status, err
		}
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	if err := contextError(ctx); err != nil {
		return s.status, err
	}
	s.autoUnlock = false
	s.status.AutoUnlockEnabled = false
	s.hint = ""
	s.status.Hint = ""

	createdPasswordWrapper := false
	createdAutoKey := false
	createdAutoWrapper := false
	createdHint := false
	cleanup := func() {
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

	passwordStore, err := s.passwordKeyStore(s.passwordKeyPath, masterPassword)
	if err != nil {
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	var writeErr error
	if passwordWrapperExists {
		replaceable, ok := passwordStore.(replaceableMasterKeyStore)
		if !ok {
			_ = passwordStore.Close()
			return s.status, ErrCredentialVaultRecoveryFailed
		}
		writeErr = replaceable.Replace(ctx, masterKey)
	} else {
		writeErr = passwordStore.Save(ctx, masterKey)
	}
	if writeErr != nil {
		_ = passwordStore.Close()
		if errors.Is(writeErr, vault.ErrMasterKeyExists) {
			return s.status, ErrCredentialVaultRecoveryConflict
		}
		if !passwordWrapperExists && artifactExists(s.passwordKeyPath) {
			_ = removeCredentialArtifact(s.passwordKeyPath)
		}
		return s.status, mapCredentialVaultWriteError(writeErr)
	}
	createdPasswordWrapper = !passwordWrapperExists
	if err := passwordStore.Close(); err != nil {
		cleanup()
		return s.status, ErrCredentialVaultRecoveryFailed
	}

	autoKey, autoKeyCreated, err := vault.LoadOrCreateAutoUnlockKeyWithStatus(s.autoKeyPath)
	if err != nil {
		cleanup()
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	createdAutoKey = autoKeyCreated
	defer clear(autoKey)
	autoStore, err := s.autoKeyStore(s.autoWrapPath, autoKey)
	if err != nil {
		cleanup()
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	if err := autoStore.Save(ctx, masterKey); err != nil {
		_ = autoStore.Close()
		if errors.Is(err, vault.ErrMasterKeyExists) {
			createdAutoKey = false
			cleanup()
			return s.status, ErrCredentialVaultRecoveryConflict
		}
		if artifactExists(s.autoWrapPath) {
			createdAutoWrapper = true
		}
		cleanup()
		return s.status, mapCredentialVaultWriteError(err)
	}
	createdAutoWrapper = true
	if err := autoStore.Close(); err != nil {
		cleanup()
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	if hint != "" {
		if err := savePasswordHintWithContext(ctx, s.hintPath, hint); err != nil {
			if artifactExists(s.hintPath) {
				createdHint = true
			}
			cleanup()
			if isContextError(err) {
				return s.status, err
			}
			return s.status, ErrCredentialVaultRecoveryFailed
		}
		createdHint = true
	}

	s.passwordSet = true
	s.autoUnlock = true
	s.hint = hint
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	keepStore = true
	return s.status, nil
}

func (s *CredentialService) exportSecurityRecovery(
	ctx context.Context,
	input ExportCredentialVaultKeyInput,
	recoveryKey []byte,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return ErrCredentialServiceNotReady
	}
	if s.security == nil || s.status.State != CredentialVaultStateUnlocked {
		return ErrCredentialVaultExportNotAllowed
	}
	if err := validateCredentialVaultPackagePath(input.Path, s.vaultPackageForbiddenPaths()...); err != nil {
		return err
	}
	if err := s.security.ExportRecovery(ctx, input.Path, recoveryKey); err != nil {
		if isContextError(err) {
			return err
		}
		return ErrCredentialVaultExportFailed
	}
	return nil
}

func (s *CredentialService) importSecurityRecovery(
	ctx context.Context,
	input ImportCredentialVaultKeyInput,
	recoveryKey []byte,
) (CredentialVaultStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return CredentialVaultStatus{}, ErrCredentialServiceNotReady
	}
	if s.security == nil || s.manager != nil || s.store != nil || s.status.State == CredentialVaultStateUnlocked {
		return s.status, ErrCredentialVaultRecoveryNotAllowed
	}
	if err := validateCredentialVaultPackagePath(input.Path, s.vaultPackageForbiddenPaths()...); err != nil {
		return s.status, err
	}
	importedDEK, err := security.ReadRecoveryPackage(ctx, input.Path, recoveryKey)
	if err != nil {
		if isContextError(err) {
			return s.status, err
		}
		if errors.Is(err, security.ErrAuthentication) || errors.Is(err, security.ErrRecoveryPackage) {
			return s.status, ErrCredentialVaultRecoveryPackageAuthentication
		}
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	defer clear(importedDEK)

	if exists, existsErr := vault.VaultDatabaseExists(s.vaultPath); existsErr != nil {
		return s.status, ErrCredentialVaultRecoveryFailed
	} else if exists {
		keys, deriveErr := security.DeriveKeys(importedDEK)
		if deriveErr != nil {
			return s.status, ErrCredentialVaultRecoveryFailed
		}
		candidateStore, openErr := vault.OpenExistingWithMasterKey(ctx, s.vaultPath, keys.Vault)
		clear(keys.Vault)
		clear(keys.Qterm)
		clear(keys.Sync)
		if openErr != nil {
			return s.status, ErrCredentialVaultRecoveryPackageAuthentication
		}
		verifyErr := candidateStore.VerifyIdentity(ctx)
		_ = candidateStore.Close()
		if verifyErr != nil {
			return s.status, ErrCredentialVaultRecoveryPackageAuthentication
		}
	}
	if err := s.security.InstallRecoveryDEK(ctx, importedDEK); err != nil {
		if isContextError(err) {
			return s.status, err
		}
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	store, err := s.openSecurityVaultLocked(ctx)
	if err != nil {
		s.security.Lock()
		s.status = securityStatusToCredentialStatus(s.security.Status())
		s.status.State = CredentialVaultStateRecovery
		s.status.Reason = "recoveryRequired"
		s.status.MessageKey = "credential.status.databaseMissing"
		return s.status, ErrCredentialVaultRecoveryFailed
	}
	s.autoUnlock = true
	s.passwordSet = s.security.Status().CustomConfigured
	s.installVaultLocked(store, CredentialVaultModeAutomatic)
	return s.status, nil
}

func credentialVaultRecoveryState(state CredentialVaultState) bool {
	return state == CredentialVaultStateLocked ||
		state == CredentialVaultStateRecovery ||
		state == CredentialVaultStateError
}

type snapshotMasterKeyStore struct {
	key     []byte
	loadErr error
}

func (s *snapshotMasterKeyStore) Load(ctx context.Context) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, vault.ErrMasterKeyUnavailable
	}
	if s.loadErr != nil {
		err := s.loadErr
		s.loadErr = nil
		return nil, err
	}
	if len(s.key) != vault.MasterKeySize {
		return nil, vault.ErrMasterKeyNotFound
	}
	return bytes.Clone(s.key), nil
}

func (s *snapshotMasterKeyStore) Save(ctx context.Context, masterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil {
		return vault.ErrMasterKeyUnavailable
	}
	if len(masterKey) != vault.MasterKeySize {
		return vault.ErrInvalidKey
	}
	if len(s.key) != 0 {
		return vault.ErrMasterKeyExists
	}
	s.key = bytes.Clone(masterKey)
	return nil
}

func (s *snapshotMasterKeyStore) clear() {
	if s == nil {
		return
	}
	clear(s.key)
	s.key = nil
}

func validateCredentialVaultPackagePath(path string, forbidden ...string) error {
	if path == "" || !filepath.IsAbs(path) {
		return ErrCredentialVaultPackagePath
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == filepath.Dir(cleanPath) {
		return ErrCredentialVaultPackagePath
	}
	for _, candidate := range forbidden {
		if candidate != "" && sameCredentialPath(cleanPath, filepath.Clean(candidate)) {
			return ErrCredentialVaultPackagePath
		}
	}
	return nil
}

func sameCredentialPath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (s *CredentialService) vaultPackageForbiddenPaths() []string {
	if s == nil {
		return nil
	}
	return []string{
		s.vaultPath,
		s.vaultPath + "-wal",
		s.vaultPath + "-shm",
		s.vaultPath + "-journal",
		s.passwordKeyPath,
		s.autoKeyPath,
		s.autoWrapPath,
		s.hintPath,
		s.lifecycleLockPath,
	}
}

func ensureCredentialVaultArtifactsAbsent(paths ...string) error {
	for _, path := range paths {
		exists, err := vaultFileExists(path)
		if err != nil {
			return ErrCredentialVaultRecoveryFailed
		}
		if exists {
			return ErrCredentialVaultRecoveryConflict
		}
	}
	return nil
}

func removeCredentialVaultArtifacts(paths ...string) error {
	var firstErr error
	for _, path := range paths {
		if err := removeCredentialArtifact(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func removeCredentialVaultArtifactsContext(ctx context.Context, paths ...string) error {
	for _, path := range paths {
		if err := contextError(ctx); err != nil {
			return err
		}
		if err := removeCredentialArtifact(path); err != nil {
			return err
		}
	}
	return nil
}

func removeCredentialArtifact(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("credential artifact is a directory")
	}
	return os.Remove(path)
}

func artifactExists(path string) bool {
	exists, err := vaultFileExists(path)
	return err == nil && exists
}

func verifyImportedVault(ctx context.Context, store *vault.Store) error {
	if err := store.VerifyIdentity(ctx); err != nil {
		return err
	}
	metadata, err := store.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range metadata {
		secret, err := store.Read(ctx, item.ID)
		clear(secret.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

func mapCredentialVaultImportError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	switch {
	case errors.Is(err, vault.ErrMasterPasswordAuthentication):
		return ErrCredentialVaultRecoveryPackageAuthentication
	case errors.Is(err, vault.ErrMasterKeyNotFound),
		errors.Is(err, vault.ErrInvalidMasterKeyWrap),
		errors.Is(err, vault.ErrUnsupportedMasterKeyWrap),
		errors.Is(err, vault.ErrInvalidKey):
		return ErrCredentialVaultRecoveryPackageInvalid
	default:
		return ErrCredentialVaultRecoveryFailed
	}
}

func mapCredentialVaultOpenError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ErrCredentialVaultRecoveryFailed
}

func mapCredentialVaultWriteError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, vault.ErrMasterKeyExists) {
		return ErrCredentialVaultRecoveryConflict
	}
	return ErrCredentialVaultRecoveryFailed
}
