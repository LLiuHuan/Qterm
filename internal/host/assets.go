package host

import (
	"context"

	"changeme/internal/assetdata"
)

func (s *HostService) ExportAssets(ctx context.Context) (assetdata.Snapshot, error) {
	if s == nil {
		return assetdata.Snapshot{}, ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return assetdata.Snapshot{}, ErrHostServiceNotReady
	}
	return s.store.ExportAssets(ctx)
}

func (s *HostService) ReplaceAssets(ctx context.Context, data assetdata.Snapshot) error {
	if s == nil {
		return ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return s.store.ReplaceAssets(ctx, data)
}
