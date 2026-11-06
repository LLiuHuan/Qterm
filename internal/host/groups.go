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

func (s *HostService) ReorderGroup(id, parentID, targetID, position string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.ReorderGroup(context.Background(), strings.TrimSpace(id), strings.TrimSpace(parentID), strings.TrimSpace(targetID), strings.TrimSpace(position)); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return errno.New(errno.CodeNotFound, "host.group_not_found", "")
		case errors.Is(err, storage.ErrConflict):
			return errno.New(errno.CodeInvalidArg, "host.group_cycle", "")
		case errors.Is(err, storage.ErrProtectedGroup):
			return errno.New(errno.CodePermissionDenied, "host.default_move", "")
		default:
			return errno.New(errno.CodeStoreFailed, "host.group_reorder_failed", "")
		}
	}
	return nil
}

func (s *HostService) MoveGroup(id, parentID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.MoveGroup(context.Background(), strings.TrimSpace(id), strings.TrimSpace(parentID)); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return errno.New(errno.CodeNotFound, "host.group_not_found", "")
		case errors.Is(err, storage.ErrConflict):
			return errno.New(errno.CodeInvalidArg, "host.group_cycle", "")
		case errors.Is(err, storage.ErrProtectedGroup):
			return errno.New(errno.CodePermissionDenied, "host.default_move", "")
		default:
			return errno.New(errno.CodeStoreFailed, "host.group_move_failed", "")
		}
	}
	return nil
}

func (s *HostService) CreateGroup(input model.GroupInput) (model.Group, error) {
	id, err := randomID("group")
	if err != nil {
		return model.Group{}, errno.New(errno.CodeUnknown, "host.group_id_failed", "")
	}
	group, err := model.NewGroup(id, input.Name, time.Now())
	if err != nil {
		return model.Group{}, err
	}
	group.ParentID = strings.TrimSpace(input.ParentID)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Group{}, ErrHostServiceNotReady
	}
	group, err = s.store.CreateGroup(context.Background(), group)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrConflict):
			return model.Group{}, errno.New(errno.CodeInvalidArg, "host.group_name_exists", "")
		case errors.Is(err, storage.ErrNotFound):
			return model.Group{}, errno.New(errno.CodeNotFound, "host.parent_not_found", "")
		default:
			return model.Group{}, errno.New(errno.CodeStoreFailed, "host.group_create_failed", "")
		}
	}
	return group, nil
}

func (s *HostService) UpdateGroup(id string, input model.GroupInput) (model.Group, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Group{}, errno.New(errno.CodeInvalidArg, "host.group_id_required", "")
	}
	group, err := model.NewGroup(id, input.Name, time.Now())
	if err != nil {
		return model.Group{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Group{}, ErrHostServiceNotReady
	}
	group, err = s.store.UpdateGroup(context.Background(), group)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return model.Group{}, errno.New(errno.CodeNotFound, "host.group_not_found", "")
		case errors.Is(err, storage.ErrConflict):
			return model.Group{}, errno.New(errno.CodeInvalidArg, "host.group_name_exists", "")
		case errors.Is(err, storage.ErrProtectedGroup):
			return model.Group{}, errno.New(errno.CodePermissionDenied, "host.default_rename", "")
		default:
			return model.Group{}, errno.New(errno.CodeStoreFailed, "host.group_update_failed", "")
		}
	}
	return group, nil
}

func (s *HostService) DeleteGroup(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errno.New(errno.CodeInvalidArg, "host.group_id_required", "")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.DeleteGroup(context.Background(), id); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return errno.New(errno.CodeNotFound, "host.group_not_found", "")
		case errors.Is(err, storage.ErrGroupNotEmpty):
			return errno.New(errno.CodeInvalidArg, "host.group_not_empty", "")
		case errors.Is(err, storage.ErrProtectedGroup):
			return errno.New(errno.CodePermissionDenied, "host.default_delete", "")
		default:
			return errno.New(errno.CodeStoreFailed, "host.group_delete_failed", "")
		}
	}
	return nil
}
