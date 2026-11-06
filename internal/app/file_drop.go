package app

import (
	"context"
	"time"

	"changeme/internal/model"
	"changeme/internal/sftp"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const fileDropEventName = "qterm:file-drop"

type fileDropEvent struct {
	ID         string   `json:"id"`
	SessionID  string   `json:"sessionId"`
	Directory  string   `json:"directory"`
	Paths      []string `json:"paths,omitempty"`
	MessageKey string   `json:"messageKey,omitempty"`
}

type fileDropSink interface {
	Name() string
	DispatchWailsEvent(event *application.CustomEvent)
}

type fileDropTarget interface {
	fileDropSink
	OnWindowEvent(eventType events.WindowEventType, callback func(event *application.WindowEvent)) func()
}

var _ fileDropTarget = (*application.WebviewWindow)(nil)

func emitFileDrop(sink fileDropSink, payload fileDropEvent) {
	sink.DispatchWailsEvent(&application.CustomEvent{
		Name: fileDropEventName,
		Data: payload,

		Sender: sink.Name(),
	})
}

func configureFileDrop(window fileDropTarget, service *Service) {
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		target := event.Context().DropTargetDetails()
		if target == nil {
			return
		}
		attrs := target.Attributes
		id := attrs["data-drop-session"]
		kind := attrs["data-drop-kind"]
		if id == "" || (kind != "terminal" && kind != "sftp") {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		session, err := service.sessions.GetSession(ctx, id)
		payload := fileDropEvent{SessionID: id, Directory: attrs["data-drop-directory"]}
		if err != nil || session.State != model.SessionStateConnected {
			payload.MessageKey = "session.disconnected"
		} else if session.Kind == model.SessionKindLocal && kind == "terminal" {
			payload.Paths = append([]string(nil), event.Context().DroppedFiles()...)
		} else if session.Kind == model.SessionKindSSH {

			payload.ID, err = service.sftp.RegisterDroppedFiles(id, event.Context().DroppedFiles())
			if err != nil {
				payload.MessageKey = "sftp.drop_limit"
			}
		} else if session.Kind == model.SessionKindTelnet {

			payload.MessageKey = "session.drop_unsupported"
		} else {
			return
		}

		emitFileDrop(window, payload)
	})
}

func (s *Service) UploadDroppedFiles(ctx context.Context, id, directory string) (sftp.DropUploadResult, error) {
	return publicValue(s.sftp.UploadDroppedFiles(ctx, id, directory))
}

func (s *Service) CancelDroppedFiles(id string) { s.sftp.CancelDroppedFiles(id) }
