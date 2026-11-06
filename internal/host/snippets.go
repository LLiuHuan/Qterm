package host

import (
	"context"
	"strings"
	"time"

	"changeme/internal/model"
)

func (s *HostService) ListSnippets() ([]model.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, ErrHostServiceNotReady
	}
	return s.store.ListSnippets(context.Background())
}

func (s *HostService) CreateSnippet(input model.SnippetInput) (model.Snippet, error) {
	return s.saveSnippet(input, "")
}

func (s *HostService) UpdateSnippet(id string, input model.SnippetInput) (model.Snippet, error) {
	return s.saveSnippet(input, id)
}

func (s *HostService) saveSnippet(input model.SnippetInput, id string) (model.Snippet, error) {
	isUpdate := strings.TrimSpace(id) != ""
	input.Name = strings.TrimSpace(input.Name)
	input.Content = strings.TrimSpace(input.Content)
	input.GroupID = strings.TrimSpace(input.GroupID)
	if input.Name == "" || input.Content == "" {
		return model.Snippet{}, model.ErrInvalidGroup
	}
	if id == "" {
		var err error
		id, err = randomID("snippet")
		if err != nil {
			return model.Snippet{}, err
		}
	} else {
		id = strings.TrimSpace(id)
	}
	now := time.Now().UTC()
	v := model.Snippet{ID: id, Name: input.Name, Content: input.Content, GroupID: input.GroupID, AssetGroupID: input.AssetGroupID, HostIDs: input.HostIDs, AutoExecute: input.AutoExecute, Icon: strings.TrimSpace(input.Icon), Color: strings.TrimSpace(input.Color), Pinned: input.Pinned, CreatedAt: now, UpdatedAt: now}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Snippet{}, ErrHostServiceNotReady
	}
	if id == "" {
		return s.store.CreateSnippet(context.Background(), v)
	}
	if input.GroupID != "" {
		if _, err := s.store.GetSnippetGroup(context.Background(), input.GroupID); err != nil {
			return model.Snippet{}, err
		}
	}
	if isUpdate {
		existing, err := s.store.GetSnippet(context.Background(), id)
		if err != nil {
			return model.Snippet{}, err
		}
		v.CreatedAt = existing.CreatedAt
		return s.store.UpdateSnippet(context.Background(), v)
	}
	return s.store.CreateSnippet(context.Background(), v)
}

func (s *HostService) DeleteSnippet(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.DeleteSnippet(context.Background(), strings.TrimSpace(id))
}
