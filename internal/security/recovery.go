package security

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
)

func CreateRecoveryPackage(dek, recoveryKey []byte) ([]byte, error) {
	return wrapDEKWithKey(dek, recoveryKey, recoveryEnvelopeMagic, recoveryAADLabel, rand.Reader)
}

func OpenRecoveryPackage(packageBytes, recoveryKey []byte) ([]byte, error) {
	dek, err := unwrapDEKWithKey(packageBytes, recoveryKey, recoveryEnvelopeMagic, recoveryAADLabel)
	if err != nil {
		if errors.Is(err, ErrAuthentication) {
			return nil, fmt.Errorf("%w: %w", ErrRecoveryPackage, ErrAuthentication)
		}
		return nil, fmt.Errorf("%w: open recovery package", ErrRecoveryPackage)
	}
	return dek, nil
}

func ExportRecoveryPackage(ctx context.Context, path string, dek, recoveryKey []byte) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateSecurityPath(path); err != nil {
		return err
	}
	packageBytes, err := CreateRecoveryPackage(dek, recoveryKey)
	if err != nil {
		return err
	}
	defer clear(packageBytes)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := replaceSecurityEnvelope(ctx, path, packageBytes, recoveryEnvelopeMagic); err != nil {
		return fmt.Errorf("write recovery package: %w", err)
	}
	return nil
}

func ImportRecoveryPackage(ctx context.Context, path string, recoveryKey []byte) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := validateSecurityPath(path); err != nil {
		return nil, err
	}
	packageBytes, err := readSecurityEnvelope(path, recoveryEnvelopeMagic)
	if err != nil {
		return nil, fmt.Errorf("read recovery package: %w", err)
	}
	defer clear(packageBytes)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	dek, err := OpenRecoveryPackage(packageBytes, recoveryKey)
	if err != nil {
		clear(dek)
		return nil, err
	}
	return dek, nil
}

func ReadRecoveryPackage(ctx context.Context, path string, recoveryKey []byte) ([]byte, error) {
	return ImportRecoveryPackage(ctx, path, recoveryKey)
}
