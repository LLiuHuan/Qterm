package vault

import (
	"context"
	"fmt"
)

func (s *Store) ReplaceAll(ctx context.Context, values []Secret) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.ready(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(values))
	names := make(map[string]string, len(values))
	for _, value := range values {
		if _, exists := seen[value.ID]; exists || value.ID == "" || len(value.Value) == 0 || len(value.Value) > 16<<20 {
			return ErrInvalidRecord
		}
		if err := validateMetadata(value.ID, value.Kind); err != nil {
			return err
		}
		name, err := normalizeSecretName(value.Name)
		if err != nil {
			return err
		}
		names[value.ID] = name
		seen[value.ID] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace vault secrets: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM secrets`); err != nil {
		return fmt.Errorf("clear vault secrets: %w", err)
	}
	now := s.now().UTC()
	for _, value := range values {
		envelope, err := s.cipher.Seal(value.ID, value.Kind, value.Value)
		if err != nil {
			return err
		}
		createdAt := value.CreatedAt.UTC()
		if value.CreatedAt.IsZero() {
			createdAt = now
		}
		updatedAt := value.UpdatedAt.UTC()
		if value.UpdatedAt.IsZero() {
			updatedAt = createdAt
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO secrets(id,name,kind,envelope,created_at,updated_at,username,public_key) VALUES(?,?,?,?,?,?,?,?)`, value.ID, names[value.ID], value.Kind, envelope, formatTime(createdAt), formatTime(updatedAt), value.Username, value.PublicKey)
		clear(envelope)
		if err != nil {
			return fmt.Errorf("insert replaced vault secret: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replaced vault secrets: %w", err)
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return fmt.Errorf("protect replaced vault sidecars: %w", err)
	}
	return nil
}
