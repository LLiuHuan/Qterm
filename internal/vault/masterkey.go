package vault

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

var (
	ErrMasterKeyNotFound    = errors.New("vault master key not found")
	ErrMasterKeyExists      = errors.New("vault master key already exists")
	ErrMasterKeyConflict    = errors.New("multiple vault master keys match the same identity")
	ErrMasterKeyUnavailable = errors.New("vault master key store is unavailable")
	ErrMasterKeyDenied      = errors.New("vault master key access was denied")
	ErrRecoveryRequired     = errors.New("vault master key recovery is required")
	ErrInvalidVaultDatabase = errors.New("invalid vault database file")
)

type MasterKeyStore interface {
	Load(ctx context.Context) ([]byte, error)
	Save(ctx context.Context, masterKey []byte) error
}

func OpenWithMasterKey(ctx context.Context, path string, masterKey []byte) (*Store, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	cipher, err := NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("initialize vault encryption: %w", err)
	}
	return Open(ctx, path, cipher)
}

func OpenExistingWithMasterKey(ctx context.Context, path string, masterKey []byte) (*Store, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	cipher, err := NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("initialize vault encryption: %w", err)
	}
	return open(ctx, path, cipher, false)
}

func OpenWithMasterKeyStore(
	ctx context.Context,
	path string,
	keyStore MasterKeyStore,
) (*Store, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	cleanPath, err := validateVaultDatabasePath(path)
	if err != nil {
		return nil, err
	}
	path = cleanPath
	if keyStore == nil {
		return nil, errors.New("vault master key store is nil")
	}

	masterKey, err := keyStore.Load(ctx)
	allowCreate := false
	switch {
	case err == nil:
		if len(masterKey) != MasterKeySize {
			clear(masterKey)
			return nil, ErrInvalidKey
		}
		exists, inspectErr := VaultDatabaseExists(path)
		if inspectErr != nil {
			clear(masterKey)
			return nil, fmt.Errorf("inspect vault database before open: %w", inspectErr)
		}
		if !exists {
			clear(masterKey)
			return nil, ErrRecoveryRequired
		}
	case errors.Is(err, ErrMasterKeyNotFound):
		clear(masterKey)
		masterKey, err = initializeMasterKey(ctx, path, keyStore)
		if err != nil {
			return nil, err
		}
		allowCreate = true
	default:
		clear(masterKey)
		return nil, fmt.Errorf("load vault master key: %w", err)
	}
	defer clear(masterKey)
	cipher, err := NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("initialize vault encryption: %w", err)
	}
	return open(ctx, path, cipher, allowCreate)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("vault context is nil")
	}
	return ctx.Err()
}

func initializeMasterKey(
	ctx context.Context,
	path string,
	keyStore MasterKeyStore,
) ([]byte, error) {
	exists, err := vaultArtifactsExist(path)
	if err != nil {
		return nil, fmt.Errorf("inspect vault database before key creation: %w", err)
	}
	if exists {
		return nil, ErrRecoveryRequired
	}

	masterKey, err := GenerateMasterKey()
	if err != nil {
		return nil, err
	}
	if err := keyStore.Save(ctx, masterKey); err != nil {
		clear(masterKey)
		if errors.Is(err, ErrMasterKeyExists) {

			winner, loadErr := keyStore.Load(ctx)
			if loadErr != nil {
				clear(winner)
				return nil, fmt.Errorf("load concurrently created vault master key: %w", loadErr)
			}
			return winner, nil
		}
		return nil, fmt.Errorf("save new vault master key: %w", err)
	}
	return masterKey, nil
}

func vaultArtifactsExist(path string) (bool, error) {
	cleanPath, err := validateVaultDatabasePath(path)
	if err != nil {
		return false, err
	}
	paths := []string{cleanPath, cleanPath + "-wal", cleanPath + "-shm", cleanPath + "-journal"}
	for _, candidate := range paths {
		_, err := os.Lstat(candidate)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return false, err
		}
	}
	return false, nil
}

func VaultDatabaseExists(path string) (bool, error) {
	cleanPath, err := validateVaultDatabasePath(path)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(cleanPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return true, nil
}

func VaultExists(path string) (bool, error) {
	return vaultArtifactsExist(path)
}
