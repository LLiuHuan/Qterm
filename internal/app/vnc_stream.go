package app

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/vnc"

	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
)

func RegisterVncStreams(application *wailsapp.App, service *Service) {
	if application == nil || service == nil {
		return
	}
	application.HandleStream(vnc.StreamName, service.handleVncStream)
}

func (s *Service) handleVncStream(stream *wailsapp.StreamConn) {
	manager, err := s.vncManager()
	if err != nil {
		return
	}
	vnc.ServeStream(stream, manager, vncStreamTokens{manager: manager})
}

func (s *Service) VncStreamTicket(sessionID string) (model.VncStreamTicket, error) {
	manager, err := s.vncManager()
	if err != nil {
		return model.VncStreamTicket{}, publicError(err)
	}
	ticket, err := manager.IssueTicket(sessionID)
	if err != nil {
		if err == vnc.ErrSessionNotFound {
			return model.VncStreamTicket{}, publicError(vncError(errno.CodeNotFound, "vnc.sessionNotFound", err))
		}
		return model.VncStreamTicket{}, publicError(vncError(errno.CodeUnknown, "vnc.streamUnavailable", err))
	}
	return ticket, nil
}

type vncStreamTokens struct{ manager *vnc.Manager }

func (t vncStreamTokens) Consume(sessionID, token string) bool {
	if t.manager == nil {
		return false
	}
	return t.manager.ConsumeToken(sessionID, token)
}
