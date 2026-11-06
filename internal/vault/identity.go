package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	vaultIdentitySingleton = 1
	vaultIdentityRecordID  = "qterm-vault-identity"
	vaultIdentitySize      = MasterKeySize
)

var (
	ErrVaultIdentityMissing        = errors.New("vault identity is missing")
	ErrVaultIdentityAuthentication = errors.New("vault identity authentication failed")
)

func (s *Store) InitializeIdentity(ctx context.Context) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := contextError(ctx); err != nil {
		return err
	}

	if err := s.VerifyIdentity(ctx); err == nil {
		return nil
	} else if !errors.Is(err, ErrVaultIdentityMissing) {
		return err
	}

	identity, err := GenerateMasterKey()
	if err != nil {
		return err
	}
	defer clear(identity)
	envelope, err := s.cipher.Seal(vaultIdentityRecordID, KindPrivateKeyPassphrase, identity)
	if err != nil {
		return fmt.Errorf("encrypt vault identity: %w", err)
	}
	defer clear(envelope)

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO vault_identity (singleton, envelope) VALUES (?, ?)`,
		vaultIdentitySingleton,
		envelope,
	); err != nil {
		if verifyErr := s.VerifyIdentity(ctx); verifyErr == nil {
			return nil
		}
		return fmt.Errorf("create vault identity: %w", err)
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return fmt.Errorf("protect vault database sidecars: %w", err)
	}
	return nil
}

func (s *Store) VerifyIdentity(ctx context.Context) error {
	if err := s.ready(); err != nil {
		return err
	}
	return s.verifyIdentityWithCipher(ctx, s.cipher)
}

func (s *Store) VerifyMasterKey(ctx context.Context, masterKey []byte) error {
	if err := s.ready(); err != nil {
		return err
	}
	cipher, err := NewCipher(masterKey)
	if err != nil {
		return err
	}
	return s.verifyIdentityWithCipher(ctx, cipher)
}

func (s *Store) verifyIdentityWithCipher(ctx context.Context, cipher *Cipher) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if cipher == nil {
		return ErrNotReady
	}

	var envelope []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT envelope FROM vault_identity WHERE singleton = ?`,
		vaultIdentitySingleton,
	).Scan(&envelope)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVaultIdentityMissing
	}
	if err != nil {
		return fmt.Errorf("read vault identity: %w", err)
	}
	defer clear(envelope)

	identity, err := cipher.Open(vaultIdentityRecordID, KindPrivateKeyPassphrase, envelope)
	if err != nil {
		if errors.Is(err, ErrAuthentication) || errors.Is(err, ErrInvalidRecord) ||
			errors.Is(err, ErrUnsupportedVersion) || errors.Is(err, ErrUnsupportedAlgorithm) {
			return ErrVaultIdentityAuthentication
		}
		return fmt.Errorf("decrypt vault identity: %w", err)
	}
	defer clear(identity)
	if len(identity) != vaultIdentitySize {
		return ErrVaultIdentityAuthentication
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return nil
}
