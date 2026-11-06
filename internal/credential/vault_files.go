package credential

import (
	"changeme/internal/errno"
	"changeme/internal/vault"
	"context"
	"crypto/subtle"
	"errors"
	"os"
)

func (s *CredentialService) loadMasterKeyWithPasswordLocked(ctx context.Context, password []byte) ([]byte, error) {
	store, err := s.passwordKeyStore(s.passwordKeyPath, password)
	if err != nil {
		return nil, err
	}
	masterKey, loadErr := store.Load(ctx)
	closeErr := store.Close()
	if loadErr != nil {
		clear(masterKey)
		if closeErr != nil {
			return nil, errors.Join(loadErr, closeErr)
		}
		return nil, loadErr
	}
	if closeErr != nil {
		clear(masterKey)
		return nil, closeErr
	}
	if err := s.verifyMasterKeyLocked(ctx, masterKey); err != nil {
		clear(masterKey)
		return nil, err
	}
	return masterKey, nil
}

func (s *CredentialService) verifyMasterKeyLocked(ctx context.Context, masterKey []byte) error {
	if s.store != nil {
		return s.store.VerifyMasterKey(ctx, masterKey)
	}
	store, err := s.openExistingVaultLocked(ctx, masterKey)
	if err != nil {
		return err
	}
	verifyErr := store.VerifyIdentity(ctx)
	closeErr := store.Close()
	return errors.Join(verifyErr, closeErr)
}

func (s *CredentialService) passwordWrapperContainsLocked(
	ctx context.Context,
	password []byte,
	expected []byte,
) bool {
	if s == nil || len(password) == 0 || len(expected) != vault.MasterKeySize {
		return false
	}
	probe, err := s.passwordKeyStore(s.passwordKeyPath, password)
	if err != nil {
		return false
	}
	key, loadErr := probe.Load(ctx)
	closeErr := probe.Close()
	defer clear(key)
	if loadErr != nil || closeErr != nil {
		return false
	}
	return bytesEqual(key, expected)
}

func (s *CredentialService) openExistingVaultLocked(ctx context.Context, masterKey []byte) (*vault.Store, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	exists, err := vault.VaultDatabaseExists(s.vaultPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, vault.ErrRecoveryRequired
	}
	return vault.OpenExistingWithMasterKey(ctx, s.vaultPath, masterKey)
}

func validatedMasterPassword(password string) ([]byte, error) {
	value := []byte(password)
	if len(value) < MinimumMasterPasswordLength {
		clear(value)
		return nil, ErrCredentialPasswordTooShort
	}
	if len(value) > 4096 {
		clear(value)
		return nil, vault.ErrInvalidMasterPassword
	}
	return value, nil
}

func bytesEqual(left, right []byte) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare(left, right) == 1
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "credential.context_missing", "")
	}
	return ctx.Err()
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func vaultFileExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func vaultRegularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func removeAutoUnlockFiles(keyPath, wrapPath string) error {

	if err := removeCredentialArtifact(wrapPath); err != nil {
		return errors.New("remove automatic unlock wrapper")
	}
	if err := removeCredentialArtifact(keyPath); err != nil {
		return errors.New("remove automatic unlock key")
	}
	return nil
}

func removeAutoUnlockFilesWithContext(ctx context.Context, keyPath, wrapPath string) error {
	return removeCredentialVaultArtifactsContext(ctx, wrapPath, keyPath)
}

func savePasswordHintWithContext(ctx context.Context, path, hint string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := vault.SavePasswordHint(path, hint); err != nil {
		return err
	}
	return contextError(ctx)
}
