package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	portableMasterKeyMagic       = "QTPK"
	portableMasterKeyVersion     = byte(1)
	portableMasterKeyHeaderSize  = len(portableMasterKeyMagic) + 1
	portableMasterKeyPackageSize = portableMasterKeyHeaderSize + masterPasswordEnvelopeSize
)

func ExportPortableMasterKey(
	ctx context.Context,
	source MasterKeyStore,
	path string,
	password []byte,
) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if source == nil {
		return errors.New("portable vault key source is nil")
	}

	masterKey, err := source.Load(ctx)
	if err != nil {
		clear(masterKey)
		return fmt.Errorf("load vault master key for export: %w", err)
	}
	defer clear(masterKey)
	if len(masterKey) != MasterKeySize {
		return ErrInvalidKey
	}

	if err := validatePasswordKeyPath(path); err != nil {
		return err
	}
	envelope, err := wrapMasterKeyWithPassword(
		ctx,
		password,
		masterKey,
		defaultMasterPasswordParameters(),
		rand.Reader,
		argon2.IDKey,
	)
	if err != nil {
		return err
	}
	defer clear(envelope)
	packageBytes := make([]byte, 0, portableMasterKeyPackageSize)
	packageBytes = append(packageBytes, portableMasterKeyMagic...)
	packageBytes = append(packageBytes, portableMasterKeyVersion)
	packageBytes = append(packageBytes, envelope...)
	defer clear(packageBytes)
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := writePortableMasterKeyPackage(ctx, path, packageBytes); err != nil {
		return fmt.Errorf("write portable vault key package: %w", err)
	}
	return nil
}

func ImportPortableMasterKey(
	ctx context.Context,
	target MasterKeyStore,
	path string,
	password []byte,
) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if target == nil {
		return errors.New("portable vault key target is nil")
	}

	if err := validatePasswordKeyPath(path); err != nil {
		return err
	}
	packageBytes, err := readPortableMasterKeyPackage(ctx, path)
	if err != nil {
		return fmt.Errorf("read portable vault key package: %w", err)
	}
	defer clear(packageBytes)
	masterKey, err := unwrapMasterKeyWithPassword(
		ctx,
		password,
		packageBytes[portableMasterKeyHeaderSize:],
		argon2.IDKey,
	)
	if err != nil {
		clear(masterKey)
		return fmt.Errorf("decrypt portable vault key package: %w", err)
	}
	defer clear(masterKey)
	return installImportedMasterKey(ctx, target, masterKey)
}

func installImportedMasterKey(ctx context.Context, target MasterKeyStore, masterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if target == nil {
		return errors.New("imported vault key target is nil")
	}
	if len(masterKey) != MasterKeySize {
		return ErrInvalidKey
	}

	existing, err := target.Load(ctx)
	switch {
	case err == nil:
		defer clear(existing)
		if sameMasterKey(existing, masterKey) {
			return nil
		}
		return ErrMasterKeyConflict
	case errors.Is(err, ErrMasterKeyNotFound):
		clear(existing)
	case err != nil:
		clear(existing)
		return fmt.Errorf("inspect local vault master key before import: %w", err)
	}

	if err := target.Save(ctx, masterKey); err != nil {
		if errors.Is(err, ErrMasterKeyExists) {

			winner, loadErr := target.Load(ctx)
			if loadErr != nil {
				clear(winner)
				return fmt.Errorf("load concurrently imported vault master key: %w", loadErr)
			}
			defer clear(winner)
			if sameMasterKey(winner, masterKey) {
				return nil
			}
			return ErrMasterKeyConflict
		}
		return fmt.Errorf("install imported vault master key: %w", err)
	}
	return nil
}

func sameMasterKey(left, right []byte) bool {
	return len(left) == MasterKeySize && len(right) == MasterKeySize &&
		subtle.ConstantTimeCompare(left, right) == 1
}

func readPortableMasterKeyPackage(ctx context.Context, path string) (packageBytes []byte, resultErr error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, portableMasterKeyPackageError("inspect portable vault key package", err)
	}
	if !info.Mode().IsRegular() || info.Size() != int64(portableMasterKeyPackageSize) {
		return nil, ErrInvalidMasterKeyWrap
	}

	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMasterKeyNotFound
	}
	if err != nil {
		return nil, portableMasterKeyPackageError("open portable vault key package", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, portableMasterKeyPackageError("close portable vault key package", err))
		}
		if resultErr != nil {
			clear(packageBytes)
			packageBytes = nil
		}
	}()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, portableMasterKeyPackageError("verify portable vault key package", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, ErrMasterKeyDenied
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return nil, portableMasterKeyPackageError("protect portable vault key package", err)
		}
	}
	packageBytes, err = io.ReadAll(io.LimitReader(file, int64(portableMasterKeyPackageSize+1)))
	if err != nil {
		return nil, portableMasterKeyPackageError("read portable vault key package", err)
	}
	if err := contextError(ctx); err != nil {
		clear(packageBytes)
		return nil, err
	}
	if len(packageBytes) != portableMasterKeyPackageSize {
		return nil, ErrInvalidMasterKeyWrap
	}
	if string(packageBytes[:len(portableMasterKeyMagic)]) != portableMasterKeyMagic {
		return nil, ErrInvalidMasterKeyWrap
	}
	if packageBytes[len(portableMasterKeyMagic)] != portableMasterKeyVersion {
		return nil, ErrUnsupportedMasterKeyWrap
	}
	return packageBytes, nil
}

func writePortableMasterKeyPackage(ctx context.Context, path string, packageBytes []byte) (resultErr error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(packageBytes) != portableMasterKeyPackageSize {
		return ErrInvalidMasterKeyWrap
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return portableMasterKeyPackageError("create portable vault key package directory", err)
	}

	temporary, err := os.CreateTemp(dir, ".qterm-portable-key-*")
	if err != nil {
		return portableMasterKeyPackageError("create temporary portable vault key package", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if temporary != nil {
			if err := temporary.Close(); err != nil {
				resultErr = errors.Join(resultErr, portableMasterKeyPackageError("close temporary portable vault key package", err))
			}
		}
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			resultErr = errors.Join(resultErr, portableMasterKeyPackageError("remove temporary portable vault key package", err))
		}
	}()

	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			return portableMasterKeyPackageError("protect portable vault key package", err)
		}
	}
	written, err := io.Copy(temporary, bytes.NewReader(packageBytes))
	if err != nil {
		return portableMasterKeyPackageError("write temporary portable vault key package", err)
	}
	if written != int64(len(packageBytes)) {
		return portableMasterKeyPackageError("write temporary portable vault key package", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return portableMasterKeyPackageError("sync temporary portable vault key package", err)
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		temporary = nil
		return portableMasterKeyPackageError("close temporary portable vault key package", err)
	}
	temporary = nil
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrMasterKeyExists
		}
		return portableMasterKeyPackageError("commit portable vault key package", err)
	}
	if err := syncPasswordKeyDirectory(dir); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return portableMasterKeyPackageError("remove committed temporary portable vault key package", err)
	}
	return nil
}

func portableMasterKeyPackageError(operation string, err error) error {
	return fmt.Errorf("%s: %v: %w", operation, err, ErrMasterKeyUnavailable)
}
