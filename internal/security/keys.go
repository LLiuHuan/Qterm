package security

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	DEKSize = 32

	MaxCustomKeySize = 4096

	MinCustomKeySize = 8
)

type KeyPurpose string

const (
	KeyPurposeVault KeyPurpose = "vault"
	KeyPurposeQterm KeyPurpose = "qterm"
	KeyPurposeSync  KeyPurpose = "sync"
)

type DerivedKeys struct {
	Vault []byte
	Qterm []byte
	Sync  []byte
}

func GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		clear(dek)
		return nil, fmt.Errorf("generate data encryption key: %w", err)
	}
	return dek, nil
}

func DeriveKey(dek []byte, purpose KeyPurpose) ([]byte, error) {
	if !validDEK(dek) {
		return nil, ErrInvalidDEK
	}
	if !validKeyPurpose(purpose) {
		return nil, fmt.Errorf("unsupported key purpose %q", purpose)
	}

	info := []byte("qterm/local-data/v1/" + string(purpose))
	derived := make([]byte, DEKSize)
	reader := hkdf.New(sha256.New, dek, nil, info)
	if _, err := io.ReadFull(reader, derived); err != nil {
		clear(derived)
		return nil, fmt.Errorf("derive %s encryption key: %w", purpose, err)
	}
	return derived, nil
}

func DeriveKeys(dek []byte) (DerivedKeys, error) {
	if !validDEK(dek) {
		return DerivedKeys{}, ErrInvalidDEK
	}
	vaultKey, err := DeriveKey(dek, KeyPurposeVault)
	if err != nil {
		return DerivedKeys{}, err
	}
	qtermKey, err := DeriveKey(dek, KeyPurposeQterm)
	if err != nil {
		clear(vaultKey)
		return DerivedKeys{}, err
	}
	syncKey, err := DeriveKey(dek, KeyPurposeSync)
	if err != nil {
		clear(vaultKey)
		clear(qtermKey)
		return DerivedKeys{}, err
	}
	return DerivedKeys{Vault: vaultKey, Qterm: qtermKey, Sync: syncKey}, nil
}

func validDEK(dek []byte) bool {
	return len(dek) == DEKSize
}

func validateCustomKey(key []byte) error {
	if len(key) < MinCustomKeySize || len(key) > MaxCustomKeySize {
		return ErrInvalidCustomKey
	}
	return nil
}

func validKeyPurpose(purpose KeyPurpose) bool {
	switch purpose {
	case KeyPurposeVault, KeyPurposeQterm, KeyPurposeSync:
		return true
	default:
		return false
	}
}

func checkContext(ctx interface{ Err() error }) error {
	if ctx == nil {
		return errors.New("security context is nil")
	}
	return ctx.Err()
}
