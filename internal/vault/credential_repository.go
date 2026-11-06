package vault

import (
	"changeme/internal/model"
	"context"
	"fmt"
)

type CredentialRepository struct {
	store *Store
}

func NewCredentialRepository(store *Store) *CredentialRepository {
	return &CredentialRepository{store: store}
}

func (r *CredentialRepository) List(ctx context.Context) ([]model.Credential, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	metadata, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	credentials := make([]model.Credential, 0, len(metadata))
	for _, item := range metadata {
		credential, err := credentialFromMetadata(item)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func (r *CredentialRepository) Create(
	ctx context.Context,
	credential model.Credential,
	value []byte,
) (model.Credential, error) {
	if err := r.ready(); err != nil {
		return model.Credential{}, err
	}
	kind, err := vaultKind(credential.Kind)
	if err != nil {
		return model.Credential{}, err
	}
	metadata, err := r.store.CreateWithMetadata(ctx, credential.ID, credential.Name, credential.Username, credential.PublicKey, kind, value)
	if err != nil {
		return model.Credential{}, err
	}
	return credentialFromMetadata(metadata)
}

func (r *CredentialRepository) Rename(
	ctx context.Context,
	id string,
	name string,
) (model.Credential, error) {
	if err := r.ready(); err != nil {
		return model.Credential{}, err
	}
	metadata, err := r.store.Rename(ctx, id, name)
	if err != nil {
		return model.Credential{}, err
	}
	return credentialFromMetadata(metadata)
}

func (r *CredentialRepository) Replace(
	ctx context.Context,
	id string,
	kind model.CredentialKind,
	value []byte,
) (model.Credential, error) {
	if err := r.ready(); err != nil {
		return model.Credential{}, err
	}
	mappedKind, err := vaultKind(kind)
	if err != nil {
		return model.Credential{}, err
	}
	metadata, err := r.store.Replace(ctx, id, mappedKind, value)
	if err != nil {
		return model.Credential{}, err
	}
	return credentialFromMetadata(metadata)
}

func (r *CredentialRepository) ReplaceWithMetadata(
	ctx context.Context,
	id string,
	input model.CredentialInput,
	value []byte,
) (model.Credential, error) {
	if err := r.ready(); err != nil {
		return model.Credential{}, err
	}
	mappedKind, err := vaultKind(input.Kind)
	if err != nil {
		return model.Credential{}, err
	}
	metadata, err := r.store.ReplaceWithMetadata(ctx, id, input.Username, input.PublicKey, mappedKind, value)
	if err != nil {
		return model.Credential{}, err
	}
	return credentialFromMetadata(metadata)
}

func (r *CredentialRepository) Read(
	ctx context.Context,
	id string,
) (model.StoredCredential, error) {
	if err := r.ready(); err != nil {
		return model.StoredCredential{}, err
	}
	secret, err := r.store.Read(ctx, id)
	if err != nil {
		return model.StoredCredential{}, err
	}
	credential, err := credentialFromMetadata(Metadata{
		ID:        secret.ID,
		Name:      secret.Name,
		Kind:      secret.Kind,
		Username:  secret.Username,
		PublicKey: secret.PublicKey,
		CreatedAt: secret.CreatedAt,
		UpdatedAt: secret.UpdatedAt,
	})
	if err != nil {
		clear(secret.Value)
		return model.StoredCredential{}, err
	}
	return model.StoredCredential{Metadata: credential, Value: secret.Value}, nil
}

func (r *CredentialRepository) Delete(ctx context.Context, id string) error {
	if err := r.ready(); err != nil {
		return err
	}
	return r.store.Delete(ctx, id)
}

func (r *CredentialRepository) ReplaceAll(ctx context.Context, values []model.StoredCredential) error {
	if err := r.ready(); err != nil {
		return err
	}
	items := make([]Secret, 0, len(values))
	for _, value := range values {
		kind, err := vaultKind(value.Metadata.Kind)
		if err != nil {
			return err
		}
		items = append(items, Secret{
			ID: value.Metadata.ID, Name: value.Metadata.Name, Kind: kind,
			Username: value.Metadata.Username, PublicKey: value.Metadata.PublicKey,
			Value: value.Value, CreatedAt: value.Metadata.CreatedAt, UpdatedAt: value.Metadata.UpdatedAt,
		})
	}
	return r.store.ReplaceAll(ctx, items)
}

func (r *CredentialRepository) ready() error {
	if r == nil || r.store == nil {
		return ErrStoreNotReady
	}
	return r.store.ready()
}

func credentialFromMetadata(metadata Metadata) (model.Credential, error) {
	kind, err := domainKind(metadata.Kind)
	if err != nil {
		return model.Credential{}, err
	}
	return model.Credential{
		ID:         metadata.ID,
		Name:       metadata.Name,
		Kind:       kind,
		Username:   metadata.Username,
		PublicKey:  metadata.PublicKey,
		Configured: true,
		CreatedAt:  metadata.CreatedAt,
		UpdatedAt:  metadata.UpdatedAt,
	}, nil
}

func vaultKind(kind model.CredentialKind) (Kind, error) {
	switch kind {
	case model.CredentialKindPassword:
		return KindPassword, nil
	case model.CredentialKindPrivateKey:
		return KindPrivateKey, nil
	case model.CredentialKindPrivateKeyPassphrase:
		return KindPrivateKeyPassphrase, nil
	case model.CredentialKindSSHAgent:
		return KindSSHAgent, nil
	default:
		return 0, fmt.Errorf("map credential kind %q: %w", kind, ErrInvalidRecord)
	}
}

func domainKind(kind Kind) (model.CredentialKind, error) {
	switch kind {
	case KindPassword:
		return model.CredentialKindPassword, nil
	case KindPrivateKey:
		return model.CredentialKindPrivateKey, nil
	case KindPrivateKeyPassphrase:
		return model.CredentialKindPrivateKeyPassphrase, nil
	case KindSSHAgent:
		return model.CredentialKindSSHAgent, nil
	default:
		return "", fmt.Errorf("map vault kind %d: %w", kind, ErrInvalidRecord)
	}
}
