package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

func (s *Service) ListCredentialPromptRules() ([]model.CredentialPromptRule, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.ListCredentialPromptRules())
}

func (s *Service) CreateCredentialPromptRule(input model.CredentialPromptRuleInput) (model.CredentialPromptRule, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.CreateCredentialPromptRule(input))
}

func (s *Service) UpdateCredentialPromptRule(id string, input model.CredentialPromptRuleInput) (model.CredentialPromptRule, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.UpdateCredentialPromptRule(id, input))
}

func (s *Service) DeleteCredentialPromptRule(id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.DeleteCredentialPromptRule(id))
}

func (s *Service) TypeCredentialIntoSession(sessionID string, credentialID string) error {
	if s == nil || s.sessions == nil {
		return publicError(errno.New(errno.CodeSessionClosed, "session.not_ready", ""))
	}
	return publicError(s.sessions.InjectCredentialIntoSession(context.Background(), sessionID, credentialID))
}
