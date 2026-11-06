package app

import (
	"context"
	"encoding/json"
	"errors"

	"changeme/internal/x11"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *Service) GetPreference(ctx context.Context, key string) (string, error) {
	if s == nil || s.preferences == nil {
		return "", publicError(errors.New("preference service is not ready"))
	}
	value, _, err := s.preferences.Get(ctx, key)
	if err != nil {
		return "", publicError(err)
	}
	return value, nil
}

func (s *Service) SetPreference(ctx context.Context, key, value string) error {
	if s == nil || s.preferences == nil {
		return publicError(errors.New("preference service is not ready"))
	}
	if err := s.preferences.Set(ctx, key, value); err != nil {
		return publicError(err)
	}
	if key == "qterm.settings.v1" {
		var settings struct {
			SessionHistoryLimit     int    `json:"sessionHistoryLimit"`
			SFTPAutoFollow          *bool  `json:"sftpAutoDirectoryFollow"`
			SFTPDirectoryFollowMode string `json:"sftpDirectoryFollowMode"`
			SFTPShowHidden          *bool  `json:"sftpShowHiddenFiles"`
			SFTPMaxTransfers        int    `json:"sftpMaxTransfers"`

			SFTPDragFolderUploadMode string `json:"sftpDragFolderUploadMode"`
			SFTPLocalDirectory       string `json:"sftpLocalDirectory"`
			SFTPEditorPath           string `json:"sftpEditorPath"`
			RecordingDirectory       string `json:"recordingDirectory"`
			X11Display               string `json:"x11Display"`
			X11XauthPath             string `json:"x11XauthPath"`
			X11AuthorityFile         string `json:"x11AuthorityFile"`

			RdpFirstFrameTimeoutSeconds *int `json:"rdpFirstFrameTimeoutSeconds"`
			RdpHostProbeTimeoutSeconds  *int `json:"rdpHostProbeTimeoutSeconds"`
		}
		if s.hosts != nil && json.Unmarshal([]byte(value), &settings) == nil && settings.SessionHistoryLimit > 0 {
			if err := s.hosts.SetSessionHistoryLimit(settings.SessionHistoryLimit); err != nil {
				return publicError(err)
			}
		}

		if s.remoteDesktops != nil {
			firstFrame, probe := rdpConnectTimeouts(
				settings.RdpFirstFrameTimeoutSeconds,
				settings.RdpHostProbeTimeoutSeconds,
			)
			s.remoteDesktops.ApplyConnectTimeouts(firstFrame, probe)
		}
		if s.sessions != nil {
			s.sessions.SetDirectoryFollowMode(settings.SFTPDirectoryFollowMode)

			s.sessions.SetX11Settings(x11.Settings{
				Display:       settings.X11Display,
				XauthPath:     settings.X11XauthPath,
				AuthorityFile: settings.X11AuthorityFile,
			})
		}

		if err := s.applyRecordingDirectory(settings.RecordingDirectory); err != nil {
			return publicError(err)
		}
		if s.sftp != nil {
			current := s.sftp.Settings()
			if settings.SFTPAutoFollow != nil {
				current.AutoDirectoryFollow = *settings.SFTPAutoFollow
			}
			if settings.SFTPShowHidden != nil {
				current.ShowHiddenFiles = *settings.SFTPShowHidden
			}
			if settings.SFTPMaxTransfers > 0 {
				current.MaxTransfers = settings.SFTPMaxTransfers
			}

			if settings.SFTPDragFolderUploadMode != "" {
				current.DragFolderUploadMode = settings.SFTPDragFolderUploadMode
			}
			current.LocalDirectory = settings.SFTPLocalDirectory
			current.EditorPath = settings.SFTPEditorPath
			if err := s.sftp.ApplySettings(current); err != nil {
				return publicError(err)
			}
		}
	}

	if app := application.Get(); app != nil {
		source := uint(0)
		if ctx != nil {

			if window, ok := ctx.Value(application.WindowKey).(application.Window); ok && window != nil {
				source = window.ID()
			}
		}

		app.Event.Emit(settingsChangedEvent, settingsChangedPayload{Key: key, Source: source})
	}
	return nil
}
