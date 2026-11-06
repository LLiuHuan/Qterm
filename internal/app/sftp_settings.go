package app

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/sftp"
)

func (s *Service) GetSFTPSettings(ctx context.Context) (sftp.Settings, error) {
	if err := contextReady(ctx); err != nil {
		return sftp.Settings{}, err
	}
	if s == nil || s.sftp == nil {
		return sftp.Settings{}, publicError(errors.New("SFTP service is not ready"))
	}
	return s.sftp.Settings(), nil
}

func (s *Service) ApplySFTPSettings(ctx context.Context, settings sftp.Settings) error {
	if err := contextReady(ctx); err != nil {
		return err
	}
	if s == nil || s.sftp == nil {
		return publicError(errors.New("SFTP service is not ready"))
	}
	return publicError(s.sftp.ApplySettings(settings))
}

func (s *Service) SelectSFTPLocalDirectorySetting(ctx context.Context) (string, error) {
	if err := contextReady(ctx); err != nil {
		return "", err
	}
	if s == nil || s.fileDialogs == nil {
		return "", publicError(errors.New("file dialog is not ready"))
	}
	value, err := s.fileDialogs.ChooseLocalDirectory()
	if err != nil {
		return "", publicError(err)
	}
	return strings.TrimSpace(value), nil
}

func (s *Service) SelectSFTPEditor(ctx context.Context) (string, error) {
	if err := contextReady(ctx); err != nil {
		return "", err
	}
	if s == nil || s.fileDialogs == nil {
		return "", publicError(errors.New("file dialog is not ready"))
	}
	value, err := s.fileDialogs.ChooseSFTPEditor()
	if err != nil {
		return "", publicError(err)
	}
	return strings.TrimSpace(value), nil
}

func (d *appFileDialogs) ChooseSFTPEditor() (string, error) {
	return d.manager.OpenFile().SetTitle("选择本地编辑器").SetButtonText("选择").CanChooseDirectories(true).CanChooseFiles(true).PromptForSingleSelection()
}

func contextReady(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return ctx.Err()
}
