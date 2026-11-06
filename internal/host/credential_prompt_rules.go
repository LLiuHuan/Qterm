package host

import (
	"context"
	"errors"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

func (s *HostService) ListCredentialPromptRules() ([]model.CredentialPromptRule, error) {
	if s == nil {
		return nil, ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, ErrHostServiceNotReady
	}
	rules, err := s.store.ListCredentialPromptRules(context.Background())
	if err != nil {
		return nil, credentialPromptRuleError(err)
	}
	return rules, nil
}

func (s *HostService) CreateCredentialPromptRule(input model.CredentialPromptRuleInput) (model.CredentialPromptRule, error) {
	if s == nil {
		return model.CredentialPromptRule{}, ErrHostServiceNotReady
	}
	id, err := randomID("promptrule")
	if err != nil {
		return model.CredentialPromptRule{}, err
	}
	rule, err := model.NewCredentialPromptRule(id, input, time.Now().UTC())
	if err != nil {
		return model.CredentialPromptRule{}, credentialPromptRuleError(err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.CredentialPromptRule{}, ErrHostServiceNotReady
	}
	created, err := s.store.CreateCredentialPromptRule(context.Background(), rule)
	if err != nil {
		return model.CredentialPromptRule{}, credentialPromptRuleError(err)
	}
	return created, nil
}

func (s *HostService) UpdateCredentialPromptRule(id string, input model.CredentialPromptRuleInput) (model.CredentialPromptRule, error) {
	if s == nil {
		return model.CredentialPromptRule{}, ErrHostServiceNotReady
	}
	rule, err := model.NewCredentialPromptRule(strings.TrimSpace(id), input, time.Now().UTC())
	if err != nil {
		return model.CredentialPromptRule{}, credentialPromptRuleError(err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.CredentialPromptRule{}, ErrHostServiceNotReady
	}
	updated, err := s.store.UpdateCredentialPromptRule(context.Background(), rule)
	if err != nil {
		return model.CredentialPromptRule{}, credentialPromptRuleError(err)
	}
	return updated, nil
}

func (s *HostService) DeleteCredentialPromptRule(id string) error {
	if s == nil {
		return ErrHostServiceNotReady
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errno.New(errno.CodeInvalidArg, "credentialPromptRule.invalid", "")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return credentialPromptRuleError(s.store.DeleteCredentialPromptRule(context.Background(), id))
}

func credentialPromptRuleError(err error) error {
	switch {
	case err == nil:
		return nil
	case errno.Extract(err) != nil:

		return err
	case errors.Is(err, model.ErrInvalidCredentialPromptRule):
		return errno.New(errno.CodeInvalidArg, "credentialPromptRule.invalid", "")
	case errors.Is(err, storage.ErrNotFound):
		return errno.New(errno.CodeNotFound, "credentialPromptRule.not_found", "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errno.New(errno.CodeStoreFailed, "credentialPromptRule.operation_failed", "")
	}
}
