package app

import (
	"context"
	"log/slog"

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

func logUpdateFailure(message, event string, err error) {
	if err == nil {
		return
	}
	key := "unknown"
	if extracted := errno.Extract(err); extracted != nil {
		key = extracted.MessageKey
	}
	slog.Error(message, "event", event, "error_key", key)
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
		logUpdateFailure("update check failed", "update.check.failed", err)
		return update.Release{}, publicError(err)
	}
	return release, nil
}

func (s *Service) UpdateDownload(ctx context.Context) error {
	if s == nil || s.updates == nil {
		return publicError(updateUnavailable())
	}
	err := s.updates.Download(ctx)
	logUpdateFailure("update download failed", "update.download.failed", err)
	return publicError(err)
}

func (s *Service) UpdateInstall(ctx context.Context) error {
	if s == nil || s.updates == nil {
		return publicError(updateUnavailable())
	}
	err := s.updates.Install(ctx)
	logUpdateFailure("update install failed", "update.install.failed", err)
	return publicError(err)
}
