package host

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

func (s *HostService) ListCommandHistory(hostID string) ([]model.CommandHistory, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return nil, errno.New(errno.CodeInvalidArg, "command.host_required", "")
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return nil, ErrHostServiceNotReady
	}
	items, err := store.ListCommandHistory(context.Background(), hostID)
	if err != nil {
		return nil, errno.Wrap(errno.CodeStoreFailed, "command.list_failed", err)
	}
	return items, nil
}

func (s *HostService) SaveCommandHistory(item model.CommandHistory) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if strings.TrimSpace(item.HostID) == "" {
		return errno.New(errno.CodeInvalidArg, "command.host_required", "")
	}
	if strings.TrimSpace(item.Command) == "" {
		return errno.New(errno.CodeInvalidArg, "command.required", "")
	}
	if err := store.SaveCommandHistory(context.Background(), item); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "command.save_failed", err)
	}
	return nil
}

func (s *HostService) DeleteCommandHistory(id string) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if err := store.DeleteCommandHistory(context.Background(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "command.not_found", "")
		}
		return errno.Wrap(errno.CodeStoreFailed, "command.delete_failed", err)
	}
	return nil
}

func (s *HostService) SuppressCommandHistory(hostID, command string) (int64, error) {
	hostID = strings.TrimSpace(hostID)
	command = strings.TrimSpace(command)
	if hostID == "" {
		return 0, errno.New(errno.CodeInvalidArg, "command.host_required", "")
	}
	if command == "" {
		return 0, errno.New(errno.CodeInvalidArg, "command.required", "")
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return 0, ErrHostServiceNotReady
	}
	count, err := store.SuppressCommandHistory(context.Background(), hostID, command)
	if err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "command.suppress_failed", err)
	}
	return count, nil
}

func (s *HostService) ClearCommandHistory(hostID string) (int64, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return 0, errno.New(errno.CodeInvalidArg, "command.host_required", "")
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return 0, ErrHostServiceNotReady
	}
	count, err := store.ClearCommandHistory(context.Background(), hostID)
	if err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "command.clear_failed", err)
	}
	return count, nil
}
