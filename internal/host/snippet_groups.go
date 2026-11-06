package host

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/store"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *HostService) ListSnippetGroups(ctx context.Context) ([]model.Group, error) {
	s.mu.RLock()
	database := s.store
	s.mu.RUnlock()
	if database == nil {
		return nil, ErrHostServiceNotReady
	}
	return database.ListSnippetGroups(ctx)
}

func (s *HostService) CreateSnippetGroup(ctx context.Context, name string) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 256 {
		return model.Group{}, errno.New(errno.CodeInvalidArg, "snippet.group_invalid", "")
	}
	id, err := randomID("snippet_group")
	if err != nil {
		return model.Group{}, fmt.Errorf("create snippet group id: %w", err)
	}
	group, err := model.NewGroup(id, name, time.Now())
	if err != nil {
		return model.Group{}, err
	}
	s.mu.RLock()
	database := s.store
	s.mu.RUnlock()
	if database == nil {
		return model.Group{}, ErrHostServiceNotReady
	}
	result, err := database.CreateSnippetGroup(ctx, group)
	if errors.Is(err, store.ErrConflict) {
		return model.Group{}, errno.New(errno.CodeInvalidArg, "snippet.group_duplicate", "")
	}
	return result, err
}

func (s *HostService) RenameSnippetGroup(ctx context.Context, id, name string) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 256 {
		return model.Group{}, errno.New(errno.CodeInvalidArg, "snippet.group_invalid", "")
	}
	s.mu.RLock()
	database := s.store
	s.mu.RUnlock()
	if database == nil {
		return model.Group{}, ErrHostServiceNotReady
	}
	group, err := database.RenameSnippetGroup(ctx, strings.TrimSpace(id), name)
	if errors.Is(err, store.ErrConflict) {
		return model.Group{}, errno.New(errno.CodeInvalidArg, "snippet.group_duplicate", "")
	}
	return group, err
}

func (s *HostService) DeleteSnippetGroup(ctx context.Context, id string) error {
	s.mu.RLock()
	database := s.store
	s.mu.RUnlock()
	if database == nil {
		return ErrHostServiceNotReady
	}
	return database.DeleteSnippetGroup(ctx, strings.TrimSpace(id))
}

func (s *HostService) ReorderSnippet(ctx context.Context, id, groupID, targetID, position string) error {
	if position != "before" && position != "after" {
		return errno.New(errno.CodeInvalidArg, "snippet.order_invalid", "")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	err := s.store.ReorderSnippet(ctx, strings.TrimSpace(id), strings.TrimSpace(groupID), strings.TrimSpace(targetID), position)
	return errno.Wrap(errno.CodeInvalidArg, "snippet.order_failed", err)
}
