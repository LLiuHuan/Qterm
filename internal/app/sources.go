package app

import (
	"context"
	"errors"
	"sync"

	"changeme/internal/credential"
	"changeme/internal/host"
	"changeme/internal/model"
	"changeme/internal/proxy"
	"changeme/internal/session"
)

type hostServiceSource struct {
	service *host.HostService
	proxies *proxy.Service

	adhoc *adHocHostRegistry
}

type adHocHostRegistry struct {
	entries sync.Map
}

func (r *adHocHostRegistry) store(entry model.Host, password string) {
	if r == nil {
		return
	}
	r.entries.Store(entry.ID, adHocHost{host: entry, password: []byte(password)})
}

func (r *adHocHostRegistry) load(id string) (adHocHost, bool) {
	if r == nil {
		return adHocHost{}, false
	}
	value, ok := r.entries.Load(id)
	if !ok {
		return adHocHost{}, false
	}
	entry, valid := value.(adHocHost)
	if !valid {
		return adHocHost{}, false
	}
	return entry, true
}

func (r *adHocHostRegistry) forget(id string) {
	if r == nil {
		return
	}

	r.entries.Delete(id)
}

type adHocHost struct {
	host     model.Host
	password []byte
}

var _ session.SessionHostSource = (*hostServiceSource)(nil)
var _ session.ProxySource = (*hostServiceSource)(nil)
var _ session.ProxyPasswordSource = (*hostServiceSource)(nil)
var _ session.HostProxyPasswordSource = (*hostServiceSource)(nil)

func (s *hostServiceSource) ProxyByID(ctx context.Context, id string) (model.ProxyRecord, error) {
	if s == nil || s.proxies == nil {
		return model.ProxyRecord{}, errors.New("proxy service is not ready")
	}
	return s.proxies.ProxyByID(ctx, id)
}

func (s *hostServiceSource) UseProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || s.proxies == nil {
		return errors.New("proxy service is not ready")
	}
	return s.proxies.UseProxyPassword(ctx, id, use)
}

func (s *hostServiceSource) UseHostProxyPassword(ctx context.Context, id string, use func([]byte) error) error {
	if s == nil || s.service == nil {
		return errors.New("host service is not ready")
	}
	return s.service.UseHostProxyPassword(ctx, id, use)
}

func (s *hostServiceSource) SessionHost(ctx context.Context, id string) (model.Host, error) {
	if entry, ok := s.adhoc.load(id); ok {
		return entry.host, nil
	}
	return host.SessionHost(s.service, ctx, id)
}

func (s *hostServiceSource) RegisterAdHocHost(entry model.Host, password string) {
	s.adhoc.store(entry, password)
}

func (s *hostServiceSource) ForgetAdHocHost(id string) {
	s.adhoc.forget(id)
}

func (s *hostServiceSource) AdHocHost(id string) (model.Host, bool) {
	entry, ok := s.adhoc.load(id)
	if !ok {
		return model.Host{}, false
	}
	return entry.host, true
}

func (s *hostServiceSource) UseHostPassword(ctx context.Context, id string, use func([]byte) error) error {
	if entry, ok := s.adhoc.load(id); ok && len(entry.password) > 0 {
		return use(entry.password)
	}
	return s.service.UseHostPassword(ctx, id, use)
}

func (s *hostServiceSource) RememberHostPassword(ctx context.Context, id string, password []byte) error {

	if _, ok := s.adhoc.load(id); ok {
		return nil
	}
	return s.service.RememberHostPassword(ctx, id, password)
}

func (s *hostServiceSource) RememberHostCredential(ctx context.Context, id, credentialID string) error {

	if _, ok := s.adhoc.load(id); ok {
		return nil
	}
	return s.service.RememberHostCredential(ctx, id, credentialID)
}

func (s *hostServiceSource) SetDetectedHostIcon(ctx context.Context, id string, icon string) error {
	return host.SetDetectedHostIcon(s.service, ctx, id, icon)
}

type credentialServiceSource struct{ service *credential.CredentialService }

func (s *credentialServiceSource) UseCredentialSecret(
	ctx context.Context,
	id string,
	use func(model.CredentialKind, []byte) error,
) error {
	return credential.UseCredentialSecret(s.service, ctx, id, use)
}

type sessionManagerSource struct{ service *session.Service }

func (s *sessionManagerSource) manager() (*session.SessionManager, error) {
	if s == nil || s.service == nil {
		return nil, errors.New("session service is not ready")
	}
	return s.service.Manager()
}

func (s *sessionManagerSource) FileSystem(ctx context.Context, id string) (session.SessionFileSystem, error) {
	manager, err := s.manager()
	if err != nil {
		return nil, err
	}
	return manager.FileSystem(ctx, id)
}

func (s *sessionManagerSource) TransferFileSystem(ctx context.Context, id string) (session.SessionTransferFileSystem, error) {
	manager, err := s.manager()
	if err != nil {
		return nil, err
	}
	return manager.TransferFileSystem(ctx, id)
}

func (s *sessionManagerSource) TransferFileSystemForSize(
	ctx context.Context,
	id string,
	expectedSize int64,
) (session.SessionTransferFileSystem, error) {
	manager, err := s.manager()
	if err != nil {
		return nil, err
	}
	return manager.TransferFileSystemForSize(ctx, id, expectedSize)
}

func (s *sessionManagerSource) Get(ctx context.Context, id string) (model.Session, error) {
	manager, err := s.manager()
	if err != nil {
		return model.Session{}, err
	}
	return manager.Get(ctx, id)
}
