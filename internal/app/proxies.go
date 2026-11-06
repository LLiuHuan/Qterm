package app

import (
	"changeme/internal/errno"
	"changeme/internal/model"
)

func (s *Service) ListProxies() ([]model.ProxyRecord, error) {
	if s == nil || s.proxies == nil {
		return nil, errno.New(errno.CodeSessionClosed, "proxy.not_ready", "")
	}
	return publicValue(s.proxies.ListProxies())
}

func (s *Service) CreateProxy(input model.ProxyInput) (model.ProxyRecord, error) {
	if s == nil || s.proxies == nil {
		return model.ProxyRecord{}, errno.New(errno.CodeSessionClosed, "proxy.not_ready", "")
	}
	return publicValue(s.proxies.CreateProxy(input))
}

func (s *Service) UpdateProxy(id string, input model.ProxyInput) (model.ProxyRecord, error) {
	if s == nil || s.proxies == nil {
		return model.ProxyRecord{}, errno.New(errno.CodeSessionClosed, "proxy.not_ready", "")
	}
	return publicValue(s.proxies.UpdateProxy(id, input))
}

func (s *Service) DeleteProxy(id string) (int, error) {
	if s == nil || s.proxies == nil {
		return 0, errno.New(errno.CodeSessionClosed, "proxy.not_ready", "")
	}
	return publicValue(s.proxies.DeleteProxy(id))
}
