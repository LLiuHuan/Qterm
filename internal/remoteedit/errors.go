package remoteedit

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
	messageKeyConflict         = "remote.edit_conflict"
	messageKeyWatchFailed      = "remote.edit_watch_failed"
	messageKeyEditorFailed     = "remote.edit_editor_failed"
	messageKeyNotFound         = "remote.edit_not_found"
	messageKeyLocalMissing     = "remote.edit_local_missing"
	messageKeyWriteFailed      = "remote.edit_write_failed"
	messageKeyReadFailed       = "remote.edit_read_failed"
	messageKeyReadUnsupported  = "remote.edit_read_unsupported"
	messageKeyWriteUnsupported = "remote.edit_write_unsupported"
	messageKeyCacheUnavailable = "remote.edit_cache_unavailable"
	messageKeyLocalWriteFailed = "remote.edit_local_write_failed"
	messageKeyTooLarge         = "remote.edit_file_too_large"
	messageKeyPathInvalid      = "sftp.remote_path_invalid"
	messageKeyRegularOnly      = "sftp.download_regular_only"
	messageKeyStatFailed       = "sftp.stat_failed"
	messageKeySessionNotFound  = "session.not_found"
	messageKeySessionClosed    = "session.disconnected"
	messageKeyPermissionDenied = "sftp.directory_permission_denied"
	messageKeyRemoteMissing    = "sftp.remote_path_not_found"
	messageKeyServiceNotReady  = "error.serviceNotReady"
	messageKeyUnknown          = "error.unknown"
	messageKeyContentInvalid   = "remote.edit_content_invalid"
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

func (m *Manager) remoteError(ctx context.Context, sessionID string, err error, fallbackKey string) error {
	if err == nil {
		return nil
	}
	if appErr := errno.Extract(err); appErr != nil {
		return appErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if sessionID != "" && m != nil && m.sessions != nil {
		if info, getErr := m.sessions.Get(ctx, sessionID); getErr == nil && info.State != model.SessionStateConnected {
			return errno.New(errno.CodeSessionClosed, messageKeySessionClosed, "")
		}
	}
	switch {
	case errors.Is(err, os.ErrPermission):

		return errno.New(errno.CodePermissionDenied, messageKeyPermissionDenied, "")
	case errors.Is(err, os.ErrNotExist):
		return errno.New(errno.CodeNotFound, messageKeyRemoteMissing, "")
	default:

		logRemoteFailure(fallbackKey, err)
		return errno.Wrap(errno.CodeConnFailed, fallbackKey, err)
	}
}

func logRemoteFailure(messageKey string, err error) {
	logFailure("remote_edit.remote_io_failed", messageKey, err)
}

func logLocalFailure(messageKey string, err error) {
	logFailure("remote_edit.local_io_failed", messageKey, err)
}

func logFailure(event string, messageKey string, err error) {
	slog.Error("remote edit operation failed",
		"event", event,
		"message_key", messageKey,
		"error", err)
}

func IsRemoteMissing(err error) bool {
	appErr := errno.Extract(err)
	return appErr != nil && appErr.Code == errno.CodeNotFound && appErr.MessageKey == messageKeyRemoteMissing
}
