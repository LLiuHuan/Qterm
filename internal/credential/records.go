package credential

import (
	"bytes"
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/vault"
	"context"
	"encoding/json"
	"errors"
	"strings"

	xssh "golang.org/x/crypto/ssh"
)

const privateKeyPayloadPrefix = "qterm-private-key-v1:"

type privateKeyPayload struct {
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase,omitempty"`
}

func (s *CredentialService) ListCredentials(ctx context.Context) ([]model.Credential, error) {
	if s == nil {
		return nil, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return nil, err
	}
	credentials, err := manager.List(ctx)
	if err != nil {
		return nil, credentialServiceError("读取凭据列表", err)
	}
	return credentials, nil
}

func (s *CredentialService) CreateCredential(
	ctx context.Context,
	input CreateCredentialInput,
) (model.Credential, error) {
	if s == nil {
		return model.Credential{}, ErrCredentialServiceNotReady
	}
	if input.Kind == model.CredentialKindPrivateKey || input.Kind == model.CredentialKindPrivateKeyPassphrase {
		key, publicKey, err := normalizePrivateKey(input.Secret, input.Passphrase)
		if err != nil {
			return model.Credential{}, err
		}
		input.Secret = string(key)
		clear(key)
		input.PublicKey = publicKey
		input.Username = ""
	}
	secret, err := credentialSecret(input.Kind, input.Secret, input.Passphrase)
	if err != nil {
		return model.Credential{}, err
	}
	defer clear(secret)
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return model.Credential{}, err
	}
	credential, err := manager.Create(
		ctx,
		model.CredentialInput{Name: input.Name, Kind: input.Kind, Username: input.Username, PublicKey: input.PublicKey},
		secret,
	)
	if err != nil {
		return model.Credential{}, credentialServiceError("创建凭据", err)
	}
	return credential, nil
}

func (s *CredentialService) RenameCredential(
	ctx context.Context,
	id string,
	input RenameCredentialInput,
) (model.Credential, error) {
	if s == nil {
		return model.Credential{}, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return model.Credential{}, err
	}
	credential, err := manager.Rename(ctx, id, input.Name)
	if err != nil {
		return model.Credential{}, credentialServiceError("重命名凭据", err)
	}
	return credential, nil
}

func (s *CredentialService) ReplaceCredentialSecret(
	ctx context.Context,
	id string,
	input ReplaceCredentialSecretInput,
) (model.Credential, error) {
	if s == nil {
		return model.Credential{}, ErrCredentialServiceNotReady
	}
	if input.Kind == model.CredentialKindPrivateKey || input.Kind == model.CredentialKindPrivateKeyPassphrase {
		key, publicKey, err := normalizePrivateKey(input.Secret, input.Passphrase)
		if err != nil {
			return model.Credential{}, err
		}
		input.Secret = string(key)
		clear(key)
		input.PublicKey = publicKey
		input.Username = ""
	}
	secret, err := credentialSecret(input.Kind, input.Secret, input.Passphrase)
	if err != nil {
		return model.Credential{}, err
	}
	defer clear(secret)
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return model.Credential{}, err
	}
	credential, err := manager.ReplaceSecretWithMetadata(ctx, id, model.CredentialInput{Kind: input.Kind, Username: input.Username, PublicKey: input.PublicKey}, secret)
	if err != nil {
		return model.Credential{}, credentialServiceError("更新凭据", err)
	}
	return credential, nil
}

func credentialSecret(kind model.CredentialKind, secret, passphrase string) ([]byte, error) {
	if kind == model.CredentialKindSSHAgent {
		canonical, err := normalizeAgentPublicKey(secret)
		if err != nil {
			return nil, ErrInvalidCredentialSecret
		}
		return []byte(canonical), nil
	}
	if kind != model.CredentialKindPrivateKey && kind != model.CredentialKindPrivateKeyPassphrase {
		if kind == model.CredentialKindPassword && strings.TrimSpace(secret) == "" {
			return nil, ErrInvalidCredentialSecret
		}
		return []byte(secret), nil
	}
	if strings.TrimSpace(secret) == "" {
		return nil, ErrInvalidCredentialSecret
	}
	key, _, err := normalizePrivateKey(secret, passphrase)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	secret = string(key)
	if passphrase == "" {
		return []byte(secret), nil
	}
	payload, err := json.Marshal(privateKeyPayload{PrivateKey: secret, Passphrase: passphrase})
	if err != nil {
		return nil, ErrInvalidCredentialSecret
	}
	return append([]byte(privateKeyPayloadPrefix), payload...), nil
}

func normalizeAgentPublicKey(value string) (string, error) {
	publicKey, _, options, rest, err := xssh.ParseAuthorizedKey([]byte(strings.TrimSpace(value)))
	if err != nil || publicKey == nil || len(options) > 0 || len(strings.TrimSpace(string(rest))) > 0 {
		return "", ErrInvalidCredentialSecret
	}
	return strings.TrimSpace(string(xssh.MarshalAuthorizedKey(publicKey))), nil
}

func DecodePrivateKeySecret(secret []byte) (privateKey, passphrase []byte, err error) {
	if !bytes.HasPrefix(secret, []byte(privateKeyPayloadPrefix)) {
		return bytes.Clone(secret), nil, nil
	}
	var payload privateKeyPayload
	if err := json.Unmarshal(secret[len(privateKeyPayloadPrefix):], &payload); err != nil || payload.PrivateKey == "" {
		return nil, nil, ErrInvalidCredentialSecret
	}
	return []byte(payload.PrivateKey), []byte(payload.Passphrase), nil
}

func (s *CredentialService) DeleteCredential(ctx context.Context, id string) error {
	if s == nil {
		return ErrCredentialServiceNotReady
	}
	if err := contextError(ctx); err != nil {
		return err
	}

	s.mu.RLock()
	links := s.credentialLinks
	s.mu.RUnlock()
	if links != nil {
		links.Lock()
		defer links.Unlock()
		if err := contextError(ctx); err != nil {
			return err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	manager, err := s.managerLocked()
	if err != nil {
		return err
	}
	if s.references != nil {
		count, err := s.references(ctx, id)
		if err != nil {
			if isContextError(err) {
				return err
			}
			return errno.New(errno.CodeStoreFailed, "credential.references_failed", "")
		}
		if count > 0 {
			return errno.New(errno.CodePermissionDenied, "credential.referenced", "")
		}
	}
	if err := manager.Delete(ctx, id); err != nil {
		return credentialServiceError("删除凭据", err)
	}
	return nil
}

func CredentialExists(s *CredentialService, ctx context.Context, id string) (bool, error) {
	if s == nil {
		return false, ErrCredentialServiceNotReady
	}
	return s.credentialExists(ctx, id)
}

func CredentialKindOf(s *CredentialService, ctx context.Context, id string) (model.CredentialKind, bool, error) {
	if s == nil {
		return "", false, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return "", false, err
	}
	credentials, err := manager.List(ctx)
	if err != nil {
		return "", false, err
	}
	for _, credential := range credentials {
		if credential.ID == id {
			return credential.Kind, true, nil
		}
	}
	return "", false, nil
}

func UseCredentialSecret(s *CredentialService, ctx context.Context, id string, use func(model.CredentialKind, []byte) error) error {
	if s == nil {
		return ErrCredentialServiceNotReady
	}
	return s.useCredentialSecret(ctx, id, use)
}

func (s *CredentialService) credentialExists(ctx context.Context, id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return false, err
	}
	credentials, err := manager.List(ctx)
	if err != nil {
		return false, err
	}
	for _, credential := range credentials {
		if credential.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *CredentialService) useCredentialSecret(
	ctx context.Context,
	id string,
	use func(model.CredentialKind, []byte) error,
) error {
	if use == nil {
		return model.ErrInvalidCredential
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return err
	}
	credentials, err := manager.List(ctx)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if credential.ID != id {
			continue
		}
		return manager.UseSecret(ctx, id, credential.Kind, func(value []byte) error {
			return use(credential.Kind, value)
		})
	}
	return vault.ErrNotFound
}

func (s *CredentialService) managerLocked() (*CredentialManager, error) {
	if s == nil || !s.started {
		return nil, ErrCredentialServiceNotReady
	}
	if s.manager == nil {
		return nil, ErrCredentialVaultLocked
	}
	return s.manager, nil
}

func credentialServiceError(action string, err error) error {
	switch {
	case errors.Is(err, model.ErrInvalidCredential):
		var validation *model.ValidationError
		if errors.As(err, &validation) {
			return validation
		}
		return errno.New(errno.CodeInvalidArg, "credential.invalid", "")
	case errors.Is(err, ErrInvalidCredentialSecret):
		return errno.New(errno.CodeInvalidArg, "credential.secret_invalid", "")
	case errors.Is(err, vault.ErrNotFound):
		return errno.New(errno.CodeNotFound, "credential.not_found", "")
	case errors.Is(err, vault.ErrAlreadyExists):
		return errno.New(errno.CodeInvalidArg, "credential.id_exists", "")
	case errors.Is(err, vault.ErrKindMismatch),
		errors.Is(err, ErrCredentialKindMismatch):
		return errno.New(errno.CodeInvalidArg, "credential.kind_immutable", "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errno.New(errno.CodeStoreFailed, "credential.operation_failed", "")
	}
}

type RevealedCredential struct {
	Kind       model.CredentialKind `json:"kind"`
	Secret     string               `json:"secret"`
	Passphrase string               `json:"passphrase,omitempty"`
	Username   string               `json:"username,omitempty"`
	PublicKey  string               `json:"publicKey,omitempty"`
}

func (s *CredentialService) RevealCredential(ctx context.Context, id string) (RevealedCredential, error) {
	if s == nil {
		return RevealedCredential{}, ErrCredentialServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager, err := s.managerLocked()
	if err != nil {
		return RevealedCredential{}, err
	}
	credentials, err := manager.List(ctx)
	if err != nil {
		return RevealedCredential{}, err
	}
	var record model.Credential
	found := false
	for _, credential := range credentials {
		if credential.ID == id {
			record = credential
			found = true
			break
		}
	}
	if !found {
		return RevealedCredential{}, vault.ErrNotFound
	}
	revealed := RevealedCredential{
		Kind:      record.Kind,
		Username:  record.Username,
		PublicKey: record.PublicKey,
	}
	err = manager.UseSecret(ctx, id, record.Kind, func(value []byte) error {
		kind := record.Kind
		if kind == model.CredentialKindPrivateKey || kind == model.CredentialKindPrivateKeyPassphrase {
			privateKey, passphrase, decodeErr := DecodePrivateKeySecret(value)
			if decodeErr != nil {
				return decodeErr
			}
			defer clear(privateKey)
			defer clear(passphrase)
			revealed.Secret = string(privateKey)
			revealed.Passphrase = string(passphrase)
			return nil
		}
		revealed.Secret = string(value)
		return nil
	})
	if err != nil {
		return RevealedCredential{}, err
	}
	return revealed, nil
}
