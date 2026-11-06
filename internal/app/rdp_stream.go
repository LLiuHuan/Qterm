package app

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/remotedesktop"

	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
)

func RegisterRdpStreams(application *wailsapp.App, service *Service) {
	if application == nil || service == nil {
		return
	}
	application.HandleStream(remotedesktop.RDPStreamName, service.handleRdpStream)
}

func (s *Service) handleRdpStream(stream *wailsapp.StreamConn) {
	remotedesktop.ServeStream(
		stream,
		rdpStreamResolver{service: s},
		s.rdpTokens,
		remotedesktop.DefaultStreamOptions(),
	)
}

func (s *Service) RdpStreamTicket(sessionID string) (model.RdpStreamTicket, error) {
	if s == nil || s.rdpTokens == nil {
		return model.RdpStreamTicket{}, publicError(errno.New(errno.CodeUnknown, "rdp.serviceNotReady", ""))
	}
	if _, err := s.rdpSession(sessionID); err != nil {
		return model.RdpStreamTicket{}, publicError(err)
	}
	token, err := s.rdpTokens.Issue(sessionID)
	if err != nil {
		return model.RdpStreamTicket{}, publicError(errno.New(errno.CodeUnknown, "rdp.streamUnavailable", ""))
	}
	return model.RdpStreamTicket{
		StreamName:      remotedesktop.RDPStreamName,
		Token:           token,
		ProtocolVersion: int(remotedesktop.RDPProtocolVersion),
	}, nil
}

type rdpStreamResolver struct{ service *Service }

func (r rdpStreamResolver) StreamSession(id string) (*remotedesktop.Session, bool) {
	if r.service == nil {
		return nil, false
	}
	manager, err := r.service.rdpManager()
	if err != nil {
		return nil, false
	}
	return manager.Get(id)
}
