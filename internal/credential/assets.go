package credential

import (
	"context"
	"errors"
	"fmt"

	"changeme/internal/assetdata"
	"changeme/internal/model"
)

func (s *CredentialService) ExportAssets(ctx context.Context) ([]assetdata.Credential, error) {
	if s == nil {
		return nil, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return nil, err
	}
	metadata, err := manager.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]assetdata.Credential, 0, len(metadata))
	for _, item := range metadata {
		copyItem := assetdata.Credential{
			ID: item.ID, Name: item.Name, Kind: item.Kind,
			Username: item.Username, PublicKey: item.PublicKey,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
		if err := manager.UseSecret(ctx, item.ID, item.Kind, func(value []byte) error {
			copyItem.Value = append([]byte(nil), value...)
			return nil
		}); err != nil {
			for i := range result {
				clear(result[i].Value)
			}
			return nil, err
		}
		result = append(result, copyItem)
	}
	return result, nil
}

func (s *CredentialService) ReplaceAssets(ctx context.Context, values []assetdata.Credential) error {
	if s == nil {
		return ErrCredentialServiceNotReady
	}
	if err := validateCredentialAssets(values); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	manager, err := s.managerLocked()
	if err != nil {
		return err
	}
	if replacer, ok := manager.repository.(interface {
		ReplaceAll(context.Context, []model.StoredCredential) error
	}); ok {
		records := make([]model.StoredCredential, 0, len(values))
		for _, item := range values {
			records = append(records, model.StoredCredential{
				Metadata: model.Credential{ID: item.ID, Name: item.Name, Kind: item.Kind, Username: item.Username, PublicKey: item.PublicKey, Configured: true, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt},
				Value:    append([]byte(nil), item.Value...),
			})
		}
		defer func() {
			for index := range records {
				clear(records[index].Value)
			}
		}()
		return replacer.ReplaceAll(ctx, records)
	}
	metadata, err := manager.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range metadata {
		if err := manager.repository.Delete(ctx, item.ID); err != nil {
			return fmt.Errorf("clear credential assets: %w", err)
		}
	}
	for _, item := range values {
		credential := model.Credential{ID: item.ID, Name: item.Name, Kind: item.Kind, Username: item.Username, PublicKey: item.PublicKey, Configured: true, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
		if _, err := manager.repository.Create(ctx, credential, item.Value); err != nil {
			return fmt.Errorf("restore credential assets: %w", err)
		}
	}
	return nil
}

func validateCredentialAssets(values []assetdata.Credential) error {
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		if _, ok := seen[item.ID]; ok || item.ID == "" || !item.Kind.Valid() || item.Name == "" || len(item.Value) == 0 || len(item.Value) > model.MaxCredentialSecretSize {
			return errors.New("invalid credential asset")
		}
		seen[item.ID] = struct{}{}
		if _, err := model.NewCredential(item.ID, model.CredentialInput{Name: item.Name, Kind: item.Kind, Username: item.Username, PublicKey: item.PublicKey}, item.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}
