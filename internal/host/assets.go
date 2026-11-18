package host

import (
	"context"
	"log/slog"

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
	if err := s.store.ReplaceAssets(ctx, data); err != nil {

		slog.Error("asset import failed", "event", "host.assets.import_failed",
			"stage", "store", "host_count", len(data.Hosts), "group_count", len(data.Groups),
			"snippet_count", len(data.Snippets), "credential_count", len(data.Credentials),
			"proxy_count", len(data.Proxies), "error", err)
		return err
	}
	return nil
}
