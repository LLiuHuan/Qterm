package credential

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"changeme/internal/model"
)

var (
	ErrInvalidCredentialSecret   = errors.New("invalid credential secret")
	ErrCredentialKindMismatch    = errors.New("credential kind does not match")
	ErrCredentialManagerNotReady = errors.New("credential manager is not ready")
)

type CredentialRepository interface {
	List(ctx context.Context) ([]model.Credential, error)
	Create(ctx context.Context, credential model.Credential, value []byte) (model.Credential, error)
	Rename(ctx context.Context, id, name string) (model.Credential, error)
	Replace(ctx context.Context, id string, kind model.CredentialKind, value []byte) (model.Credential, error)
	Read(ctx context.Context, id string) (model.StoredCredential, error)
	Delete(ctx context.Context, id string) error
}

type CredentialManager struct {
	repository CredentialRepository
	now        func() time.Time
	newID      func() (string, error)
}

func NewCredentialManager(repository CredentialRepository) *CredentialManager {
	return newCredentialManager(repository, time.Now, randomCredentialID)
}

func newCredentialManager(
	repository CredentialRepository,
	now func() time.Time,
	newID func() (string, error),
) *CredentialManager {
	return &CredentialManager{repository: repository, now: now, newID: newID}
}

func (m *CredentialManager) List(ctx context.Context) ([]model.Credential, error) {
	if err := m.ready(ctx); err != nil {
		return nil, err
	}
	credentials, err := m.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	return append([]model.Credential(nil), credentials...), nil
}

func (m *CredentialManager) Create(
	ctx context.Context,
	input model.CredentialInput,
	value []byte,
) (model.Credential, error) {
	if err := m.ready(ctx); err != nil {
		return model.Credential{}, err
	}
	valueCopy, err := copyCredentialSecret(value)
	if err != nil {
		return model.Credential{}, err
	}
	defer clear(valueCopy)

	id, err := m.newID()
	if err != nil {
		return model.Credential{}, fmt.Errorf("generate credential ID: %w", err)
	}
	credential, err := model.NewCredential(id, input, m.now())
	if err != nil {
		return model.Credential{}, err
	}
	credential, err = m.repository.Create(ctx, credential, valueCopy)
	if err != nil {
		return model.Credential{}, fmt.Errorf("create credential: %w", err)
	}
	return credential, nil
}

func (m *CredentialManager) Rename(
	ctx context.Context,
	id string,
	name string,
) (model.Credential, error) {
	if err := m.ready(ctx); err != nil {
		return model.Credential{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Credential{}, model.ErrInvalidCredential
	}
	name, err := model.NormalizeCredentialName(name)
	if err != nil {
		return model.Credential{}, err
	}
	credential, err := m.repository.Rename(ctx, id, name)
	if err != nil {
		return model.Credential{}, fmt.Errorf("rename credential: %w", err)
	}
	return credential, nil
}

func (m *CredentialManager) ReplaceSecret(
	ctx context.Context,
	id string,
	kind model.CredentialKind,
	value []byte,
) (model.Credential, error) {
	return m.ReplaceSecretWithMetadata(ctx, id, model.CredentialInput{Kind: kind}, value)
}

func (m *CredentialManager) ReplaceSecretWithMetadata(
	ctx context.Context,
	id string,
	input model.CredentialInput,
	value []byte,
) (model.Credential, error) {
	if err := m.ready(ctx); err != nil {
		return model.Credential{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" || !input.Kind.Valid() {
		return model.Credential{}, model.ErrInvalidCredential
	}
	valueCopy, err := copyCredentialSecret(value)
	if err != nil {
		return model.Credential{}, err
	}
	defer clear(valueCopy)
	var credential model.Credential
	if replacer, ok := m.repository.(interface {
		ReplaceWithMetadata(context.Context, string, model.CredentialInput, []byte) (model.Credential, error)
	}); ok {
		credential, err = replacer.ReplaceWithMetadata(ctx, id, input, valueCopy)
	} else {
		credential, err = m.repository.Replace(ctx, id, input.Kind, valueCopy)
	}
	if err != nil {
		return model.Credential{}, fmt.Errorf("replace credential secret: %w", err)
	}
	return credential, nil
}

func (m *CredentialManager) Delete(ctx context.Context, id string) error {
	if err := m.ready(ctx); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.ErrInvalidCredential
	}
	if err := m.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) UseSecret(
	ctx context.Context,
	id string,
	expectedKind model.CredentialKind,
	use func(value []byte) error,
) error {
	if err := m.ready(ctx); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" || !expectedKind.Valid() || use == nil {
		return model.ErrInvalidCredential
	}

	stored, err := m.repository.Read(ctx, id)
	if err != nil {
		clear(stored.Value)
		return fmt.Errorf("read credential for use: %w", err)
	}
	defer clear(stored.Value)
	if stored.Metadata.Kind != expectedKind {
		return ErrCredentialKindMismatch
	}
	if len(stored.Value) == 0 || len(stored.Value) > model.MaxCredentialSecretSize {
		return ErrInvalidCredentialSecret
	}
	if err := use(stored.Value); err != nil {
		return fmt.Errorf("use credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) ready(ctx context.Context) error {
	if ctx == nil {
		return errors.New("credential context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.repository == nil || m.now == nil || m.newID == nil {
		return ErrCredentialManagerNotReady
	}
	return nil
}

func copyCredentialSecret(value []byte) ([]byte, error) {
	if len(value) == 0 || len(value) > model.MaxCredentialSecretSize {
		return nil, ErrInvalidCredentialSecret
	}
	return bytes.Clone(value), nil
}

func randomCredentialID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "credential_" + hex.EncodeToString(value[:]), nil
}
