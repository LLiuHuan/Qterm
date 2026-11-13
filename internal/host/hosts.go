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

func (s *HostService) CreateHost(input model.HostInput) (model.Host, error) {
	hostID, err := randomID("host")
	if err != nil {
		return model.Host{}, errno.New(errno.CodeUnknown, "host.id_failed", "")
	}

	now := time.Now()
	host, err := model.NewHost(hostID, input, now)
	if err != nil {
		return model.Host{}, err
	}
	var group model.Group
	if host.Group != "" {
		groupID, err := randomID("group")
		if err != nil {
			return model.Host{}, errno.New(errno.CodeUnknown, "host.group_id_failed", "")
		}
		group, err = model.NewGroup(groupID, host.Group, now)
		if err != nil {
			return model.Host{}, err
		}
	}
	unlockCredentialLink, err := s.lockCredentialLink(context.Background(), host.CredentialID)
	if err != nil {
		return model.Host{}, err
	}
	defer unlockCredentialLink()

	if err := s.requireProxyExists(context.Background(), host.ProxyID); err != nil {
		return model.Host{}, err
	}

	if err := s.validateJumpChain(context.Background(), host.ID, stringValue(host.JumpHostID)); err != nil {
		return model.Host{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Host{}, ErrHostServiceNotReady
	}
	password := []byte(input.Password)
	var proxyPassword []byte
	if hostUsesInlineProxy(host) {
		proxyPassword = []byte(input.ProxyPassword)
	}
	if host.CredentialID != "" {
		clear(password)
		password = nil
	}
	host, err = s.store.CreateHostWithPasswords(context.Background(), host, group, password, proxyPassword)
	clear(password)
	clear(proxyPassword)
	if err != nil {
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return model.Host{}, errno.New(errno.CodePermissionDenied, "host.password_unavailable", "")
		}
		return model.Host{}, errno.New(errno.CodeStoreFailed, "host.save_failed", "")
	}
	return host, nil
}

func (s *HostService) UpdateHost(id string, input model.HostInput) (model.Host, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Host{}, errno.New(errno.CodeInvalidArg, "host.id_required", "")
	}

	now := time.Now()
	host, err := model.NewHost(id, input, now)
	if err != nil {
		return model.Host{}, err
	}
	var group model.Group
	if host.Group != "" {
		groupID, err := randomID("group")
		if err != nil {
			return model.Host{}, errno.New(errno.CodeUnknown, "host.group_id_failed", "")
		}
		group, err = model.NewGroup(groupID, host.Group, now)
		if err != nil {
			return model.Host{}, err
		}
	}

	unlockCredentialLink, err := s.lockCredentialLinkForUpdate(context.Background(), id, host.CredentialID)
	if err != nil {
		return model.Host{}, err
	}
	defer unlockCredentialLink()

	if err := s.requireProxyExists(context.Background(), host.ProxyID); err != nil {
		return model.Host{}, err
	}
	if err := s.validateJumpChain(context.Background(), id, stringValue(host.JumpHostID)); err != nil {
		return model.Host{}, err
	}

	clearProxyPassword := input.ClearProxyPassword || !hostUsesInlineProxy(host)
	if hostUsesInlineProxy(host) && input.ProxyPassword == "" && !clearProxyPassword {
		s.mu.RLock()
		if s.store == nil {
			s.mu.RUnlock()
			return model.Host{}, ErrHostServiceNotReady
		}
		existing, readErr := s.store.GetHost(context.Background(), id)
		s.mu.RUnlock()
		if readErr != nil {
			if errors.Is(readErr, storage.ErrNotFound) {
				return model.Host{}, errno.New(errno.CodeNotFound, "host.not_found", "")
			}
			return model.Host{}, errno.New(errno.CodeStoreFailed, "host.read_failed", "")
		}
		if existing.ProxyPasswordConfigured && stringValue(host.ProxyUsername) == "" {
			return model.Host{}, &model.ValidationError{
				Field:  "proxyUsername",
				Reason: "配置代理密码时不能为空",
				Err:    model.ErrInvalidHost,
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Host{}, ErrHostServiceNotReady
	}
	password := []byte(input.Password)
	var proxyPassword []byte
	if hostUsesInlineProxy(host) {
		proxyPassword = []byte(input.ProxyPassword)
	}
	if host.CredentialID != "" {
		clear(password)
		password = nil
	}
	host, err = s.store.UpdateHostWithPasswords(context.Background(), host, group, password, input.ClearPassword || host.CredentialID != "", proxyPassword, clearProxyPassword)
	clear(password)
	clear(proxyPassword)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.Host{}, errno.New(errno.CodeNotFound, "host.not_found", "")
		}
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return model.Host{}, errno.New(errno.CodePermissionDenied, "host.password_unavailable", "")
		}
		return model.Host{}, errno.New(errno.CodeStoreFailed, "host.update_failed", "")
	}
	return host, nil
}

func hostUsesInlineProxy(host model.Host) bool {
	if strings.TrimSpace(host.ProxyID) != "" || host.ProxyMode == nil {
		return false
	}
	return strings.TrimSpace(*host.ProxyMode) != "" && strings.TrimSpace(*host.ProxyMode) != model.ProxyModeNone
}

func (s *HostService) DuplicateHost(id, name string) (model.Host, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Host{}, errno.New(errno.CodeInvalidArg, "host.id_required", "")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Host{}, &model.ValidationError{
			Field:  "name",
			Reason: "不能为空",
			Err:    model.ErrInvalidHost,
		}
	}

	newHostID, err := randomID("host")
	if err != nil {
		return model.Host{}, errno.New(errno.CodeUnknown, "host.id_failed", "")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.Host{}, ErrHostServiceNotReady
	}

	duplicated, err := s.store.DuplicateHost(context.Background(), id, newHostID, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.Host{}, errno.New(errno.CodeNotFound, "host.not_found", "")
		}
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return model.Host{}, errno.New(errno.CodePermissionDenied, "host.password_unavailable", "")
		}
		return model.Host{}, errno.New(errno.CodeStoreFailed, "host.save_failed", "")
	}
	return duplicated, nil
}

func (s *HostService) DeleteHost(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errno.New(errno.CodeInvalidArg, "host.id_required", "")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}

	referenceCount, err := s.store.CountHostsByJumpHostID(context.Background(), id)
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "host.read_failed", "")
	}
	if referenceCount > 0 {
		return jumpHostInUseError(referenceCount)
	}
	if err := s.store.DeleteHost(context.Background(), id); err != nil {

		if errors.Is(err, storage.ErrJumpHostReferenced) {
			latest, countErr := s.store.CountHostsByJumpHostID(context.Background(), id)
			if countErr == nil && latest > 0 {
				return jumpHostInUseError(latest)
			}
			return jumpHostInUseError(0)
		}
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "host.not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "host.delete_failed", "")
	}
	return nil
}

func (s *HostService) ReorderHost(id, groupID, targetID, position string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.ReorderHost(context.Background(), strings.TrimSpace(id), strings.TrimSpace(groupID), strings.TrimSpace(targetID), strings.TrimSpace(position)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "host.target_not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "host.reorder_failed", "")
	}
	return nil
}

func (s *HostService) MoveHost(id, groupID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.MoveHost(context.Background(), strings.TrimSpace(id), strings.TrimSpace(groupID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "host.target_not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "host.move_failed", "")
	}
	return nil
}

func SetDetectedHostIcon(s *HostService, ctx context.Context, id string, icon string) error {
	if s == nil {
		return ErrHostServiceNotReady
	}
	return s.setDetectedHostIcon(ctx, id, icon)
}

func CredentialReferenceCount(s *HostService, ctx context.Context, id string) (int, error) {
	if s == nil {
		return 0, ErrHostServiceNotReady
	}
	return s.credentialReferenceCount(ctx, id)
}

func SessionHost(s *HostService, ctx context.Context, id string) (model.Host, error) {
	if s == nil {
		return model.Host{}, ErrHostServiceNotReady
	}
	return s.sessionHost(ctx, id)
}

func (s *HostService) UseHostPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || use == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.UseHostPassword(ctx, id, use)
}

func (s *HostService) RememberHostPassword(ctx context.Context, id string, password []byte) error {
	if s == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.RememberHostPassword(ctx, id, password)
}

func (s *HostService) RememberHostCredential(ctx context.Context, id, credentialID string) error {
	id = strings.TrimSpace(id)
	credentialID = strings.TrimSpace(credentialID)
	if s == nil || id == "" || credentialID == "" {
		return ErrHostServiceNotReady
	}
	unlockCredentialLink, err := s.lockCredentialLinkForUpdate(ctx, id, credentialID)
	if err != nil {
		return err
	}
	defer unlockCredentialLink()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.RememberHostCredential(ctx, id, credentialID)
}

func (s *HostService) setDetectedHostIcon(ctx context.Context, id string, icon string) error {
	if s == nil || ctx == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.SetHostIcon(ctx, id, icon)
}
