package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) Rekey(ctx context.Context, newMasterKey []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.ready(); err != nil {
		return err
	}
	newCipher, err := NewCipher(newMasterKey)
	if err != nil {
		return err
	}
	if err := s.VerifyIdentity(ctx); err != nil {
		return err
	}

	type record struct {
		id       string
		kind     Kind
		envelope []byte
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, envelope FROM secrets`)
	if err != nil {
		return fmt.Errorf("read vault records for rekey: %w", err)
	}
	defer rows.Close()
	records := make([]record, 0)
	for rows.Next() {
		var item record
		if err := rows.Scan(&item.id, &item.kind, &item.envelope); err != nil {
			return fmt.Errorf("scan vault record for rekey: %w", err)
		}
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate vault records for rekey: %w", err)
	}

	type sealedRecord struct {
		id       string
		kind     Kind
		envelope []byte
	}
	sealed := make([]sealedRecord, 0, len(records))
	for _, item := range records {
		plaintext, openErr := s.cipher.Open(item.id, item.kind, item.envelope)
		if openErr != nil {
			return fmt.Errorf("decrypt vault record for rekey: %w", openErr)
		}
		envelope, sealErr := newCipher.Seal(item.id, item.kind, plaintext)
		clear(plaintext)
		if sealErr != nil {
			return fmt.Errorf("encrypt vault record for rekey: %w", sealErr)
		}
		sealed = append(sealed, sealedRecord{id: item.id, kind: item.kind, envelope: envelope})
	}
	identityEnvelope, err := s.readIdentityEnvelope(ctx)
	if err != nil {
		return err
	}
	identity, err := s.cipher.Open(vaultIdentityRecordID, KindPrivateKeyPassphrase, identityEnvelope)
	clear(identityEnvelope)
	if err != nil {
		return fmt.Errorf("decrypt vault identity for rekey: %w", err)
	}
	newIdentityEnvelope, err := newCipher.Seal(vaultIdentityRecordID, KindPrivateKeyPassphrase, identity)
	clear(identity)
	if err != nil {
		return fmt.Errorf("encrypt vault identity for rekey: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		clear(newIdentityEnvelope)
		return fmt.Errorf("begin vault rekey: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range sealed {
		if _, err := tx.ExecContext(ctx, `UPDATE secrets SET envelope = ? WHERE id = ? AND kind = ?`, item.envelope, item.id, item.kind); err != nil {
			clear(item.envelope)
			clear(newIdentityEnvelope)
			return fmt.Errorf("write vault record during rekey: %w", err)
		}
		clear(item.envelope)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE vault_identity SET envelope = ? WHERE singleton = ?`, newIdentityEnvelope, vaultIdentitySingleton); err != nil {
		clear(newIdentityEnvelope)
		return fmt.Errorf("write vault identity during rekey: %w", err)
	}
	clear(newIdentityEnvelope)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vault rekey: %w", err)
	}

	s.cipher = newCipher
	return s.protectSidecarsAfterWrite(ctx)
}

func (s *Store) readIdentityEnvelope(ctx context.Context) ([]byte, error) {
	var envelope []byte
	err := s.db.QueryRowContext(ctx, `SELECT envelope FROM vault_identity WHERE singleton = ?`, vaultIdentitySingleton).Scan(&envelope)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrVaultIdentityMissing
	}
	if err != nil {
		return nil, fmt.Errorf("read vault identity for rekey: %w", err)
	}
	return envelope, nil
}
