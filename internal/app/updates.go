package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/update"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const UpdateStatusEvent = "update:status"

const updateRepository = "LLiuHuan/Qterm"

func newUpdateService() *update.Service {
	return update.NewService(update.Options{
		Repository: updateRepository,
		Emit: func(status update.Status) {
			if app := application.Get(); app != nil {
				app.Event.Emit(UpdateStatusEvent, status)
			}
		},
	})
}

func updateUnavailable() error {
	return errno.New(errno.CodeUnknown, "update.unavailable", "")
}

func (s *Service) UpdateStatus() update.Status {
	if s == nil || s.updates == nil {
		return update.Status{State: update.StateIdle}
	}
	return s.updates.Status()
}

func (s *Service) UpdateCheck(ctx context.Context, currentVersion string) (update.Release, error) {
	if s == nil || s.updates == nil {
		return update.Release{}, publicError(updateUnavailable())
	}
	release, err := s.updates.Check(ctx, currentVersion)
	if err != nil {
		return update.Release{}, publicError(err)
	}
	return release, nil
}

func (s *Service) UpdateDownload(ctx context.Context) error {
	if s == nil || s.updates == nil {
		return publicError(updateUnavailable())
	}
	return publicError(s.updates.Download(ctx))
}

func (s *Service) UpdateInstall(ctx context.Context) error {
	if s == nil || s.updates == nil {
		return publicError(updateUnavailable())
	}
	return publicError(s.updates.Install(ctx))
}
