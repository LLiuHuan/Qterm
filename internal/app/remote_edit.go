package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

func (s *Service) ListLocalEdits() ([]model.LocalEdit, error) {
	if s == nil || s.remoteEdits == nil {
		return []model.LocalEdit{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return s.remoteEdits.List(), nil
}

func (s *Service) OpenLocalEdit(sessionID string, remotePath string) (model.LocalEdit, error) {
	if s == nil || s.remoteEdits == nil {
		return model.LocalEdit{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.remoteEdits.Open(context.Background(), sessionID, remotePath))
}

func (s *Service) SyncLocalEdit(editID string) (model.LocalEdit, error) {
	if s == nil || s.remoteEdits == nil {
		return model.LocalEdit{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.remoteEdits.Sync(context.Background(), editID))
}

func (s *Service) DiscardLocalEdit(editID string) error {
	if s == nil || s.remoteEdits == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicError(s.remoteEdits.Discard(editID))
}

func (s *Service) ReadRemoteFileText(sessionID string, remotePath string) (model.RemoteFileContent, error) {
	if s == nil || s.remoteEdits == nil {
		return model.RemoteFileContent{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.remoteEdits.ReadText(context.Background(), sessionID, remotePath))
}

func (s *Service) WriteRemoteFileText(
	sessionID string,
	remotePath string,
	contentBase64 string,
	expectedHash string,
) (model.RemoteFileWriteResult, error) {
	if s == nil || s.remoteEdits == nil {
		return model.RemoteFileWriteResult{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.remoteEdits.WriteText(context.Background(), sessionID, remotePath, contentBase64, expectedHash))
}
