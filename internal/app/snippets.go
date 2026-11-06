package app

import (
	"changeme/internal/model"
	"context"
)

func (s *Service) ListSnippetGroups(ctx context.Context) ([]model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.ListSnippetGroups(ctx))
}

func (s *Service) CreateSnippetGroup(ctx context.Context, name string) (model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.CreateSnippetGroup(ctx, name))
}

func (s *Service) RenameSnippetGroup(ctx context.Context, id, name string) (model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.RenameSnippetGroup(ctx, id, name))
}

func (s *Service) DeleteSnippetGroup(ctx context.Context, id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.DeleteSnippetGroup(ctx, id))
}

func (s *Service) ReorderSnippet(ctx context.Context, id, groupID, targetID, position string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.ReorderSnippet(ctx, id, groupID, targetID, position))
}
