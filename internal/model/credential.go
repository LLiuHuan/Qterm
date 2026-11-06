package model

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxCredentialNameSize   = 256
	MaxCredentialSecretSize = 16 << 20
)

type CredentialKind string

const (
	CredentialKindPassword             CredentialKind = "password"
	CredentialKindPrivateKey           CredentialKind = "privateKey"
	CredentialKindPrivateKeyPassphrase CredentialKind = "privateKeyPassphrase"
	CredentialKindSSHAgent             CredentialKind = "sshAgent"
)

type CredentialInput struct {
	Name      string         `json:"name"`
	Kind      CredentialKind `json:"kind"`
	Username  string         `json:"username,omitempty"`
	PublicKey string         `json:"publicKey,omitempty"`
}

type Credential struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Kind       CredentialKind `json:"kind"`
	Username   string         `json:"username,omitempty"`
	PublicKey  string         `json:"publicKey,omitempty"`
	Configured bool           `json:"configured"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

func NewCredential(id string, input CredentialInput, now time.Time) (Credential, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Credential{}, credentialValidationError("id", "不能为空")
	}
	name, err := NormalizeCredentialName(input.Name)
	if err != nil {
		return Credential{}, err
	}
	if !input.Kind.Valid() {
		return Credential{}, credentialValidationError("kind", "不受支持")
	}
	username, err := normalizeCredentialMetadata(input.Username, "username")
	if err != nil {
		return Credential{}, err
	}
	publicKey, err := normalizeCredentialMetadata(input.PublicKey, "publicKey")
	if err != nil {
		return Credential{}, err
	}
	if input.Kind == CredentialKindSSHAgent && publicKey == "" {
		return Credential{}, credentialValidationError("publicKey", "不能为空")
	}

	now = now.UTC()
	return Credential{
		ID:         id,
		Name:       name,
		Kind:       input.Kind,
		Username:   username,
		PublicKey:  publicKey,
		Configured: true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func NormalizeCredentialName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", credentialValidationError("name", "不能为空")
	}
	if len(name) > maxCredentialNameSize {
		return "", credentialValidationError("name", "长度不能超过 256 字节")
	}
	if !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return "", credentialValidationError("name", "包含无效字符")
	}
	return name, nil
}

func (k CredentialKind) Valid() bool {
	return k == CredentialKindPassword ||
		k == CredentialKindPrivateKey ||
		k == CredentialKindPrivateKeyPassphrase ||
		k == CredentialKindSSHAgent
}

func normalizeCredentialMetadata(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	limit := maxCredentialNameSize

	if field == "publicKey" {
		limit = 16 << 10
	}
	if len(value) > limit || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", credentialValidationError(field, "格式无效")
	}
	return value, nil
}

func credentialValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason, Err: ErrInvalidCredential}
}
