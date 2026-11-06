package preview

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const (
	MessageKeyUnsupported = "remote.preview_unsupported"

	MessageKeyFileTooLarge = "remote.preview_file_too_large"

	messageKeyPathInvalid      = "sftp.remote_path_invalid"
	messageKeyRegularOnly      = "sftp.download_regular_only"
	messageKeyStatFailed       = "sftp.stat_failed"
	messageKeyReadFailed       = "remote.edit_read_failed"
	messageKeySessionNotFound  = "session.not_found"
	messageKeySessionClosed    = "session.disconnected"
	messageKeyPermissionDenied = "sftp.directory_permission_denied"
	messageKeyRemoteMissing    = "sftp.remote_path_not_found"
	messageKeyServiceNotReady  = "error.serviceNotReady"
)

func sessionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sessionapp.ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, messageKeySessionNotFound, "")
	case errors.Is(err, sessionapp.ErrSessionNotConnected),
		errors.Is(err, sessionapp.ErrSessionFileSystemUnsupported):
		return errno.New(errno.CodeSessionClosed, messageKeySessionClosed, "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errno.Wrap(errno.CodeConnFailed, messageKeyStatFailed, err)
	}
}

func (s *Service) remoteError(ctx context.Context, sessionID string, err error, fallbackKey string) error {
	if err == nil {
		return nil
	}
	if appErr := errno.Extract(err); appErr != nil {
		return appErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if sessionID != "" && s != nil && s.sessions != nil {
		if info, getErr := s.sessions.Get(ctx, sessionID); getErr == nil && info.State != model.SessionStateConnected {
			return errno.New(errno.CodeSessionClosed, messageKeySessionClosed, "")
		}
	}
	switch {
	case errors.Is(err, os.ErrPermission):
		return errno.New(errno.CodePermissionDenied, messageKeyPermissionDenied, "")
	case errors.Is(err, os.ErrNotExist):
		return errno.New(errno.CodeNotFound, messageKeyRemoteMissing, "")
	default:

		slog.Error("remote preview operation failed",
			"event", "preview.remote_io_failed",
			"message_key", fallbackKey,
			"error", err)
		return errno.Wrap(errno.CodeConnFailed, fallbackKey, err)
	}
}
