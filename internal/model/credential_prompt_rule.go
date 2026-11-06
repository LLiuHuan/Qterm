package model

import (
	"strings"
	"time"
	"unicode/utf8"
)

const maxCredentialPromptPatternSize = 512

const (
	CredentialPromptKindUsername = "username"

	CredentialPromptKindPassword = "password"
)

type CredentialPromptRule struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Pattern   string    `json:"pattern"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CredentialPromptRuleInput struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
	Enabled *bool  `json:"enabled,omitempty"`
}

func NewCredentialPromptRule(id string, input CredentialPromptRuleInput, now time.Time) (CredentialPromptRule, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > maxCredentialIDSize || !utf8.ValidString(id) || strings.IndexByte(id, 0) >= 0 {
		return CredentialPromptRule{}, credentialPromptRuleValidationError("id", "格式无效")
	}
	kind, err := NormalizeCredentialPromptKind(input.Kind)
	if err != nil {
		return CredentialPromptRule{}, err
	}
	pattern, err := NormalizeCredentialPromptPattern(input.Pattern)
	if err != nil {
		return CredentialPromptRule{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now = now.UTC()
	return CredentialPromptRule{
		ID:        id,
		Kind:      kind,
		Pattern:   pattern,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func NormalizeCredentialPromptKind(kind string) (string, error) {
	kind = strings.TrimSpace(kind)
	if kind != CredentialPromptKindUsername && kind != CredentialPromptKindPassword {
		return "", credentialPromptRuleValidationError("kind", "必须是 username 或 password")
	}
	return kind, nil
}

func NormalizeCredentialPromptPattern(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", credentialPromptRuleValidationError("pattern", "不能为空")
	}
	if !utf8.ValidString(pattern) || strings.IndexByte(pattern, 0) >= 0 {
		return "", credentialPromptRuleValidationError("pattern", "包含无效字符")
	}
	if len(pattern) > maxCredentialPromptPatternSize {
		return "", credentialPromptRuleValidationError("pattern", "长度不能超过 512 字节")
	}
	return pattern, nil
}

func credentialPromptRuleValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason, Err: ErrInvalidCredentialPromptRule}
}
