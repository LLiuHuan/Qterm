package app

import (
	"changeme/internal/terminal"
	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
)

func RegisterSessionStreams(app *wailsapp.App, service *Service) {
	if app == nil || service == nil {
		return
	}
	app.HandleStream(terminal.TerminalStreamName, service.handleTerminalStream)
}

func (s *Service) handleTerminalStream(stream *wailsapp.StreamConn) {
	manager, err := s.sessions.Manager()
	if err != nil {
		_ = stream.Send(terminal.OpenErrorFrame("会话服务尚未就绪"))
		_ = stream.Close()
		return
	}
	terminal.ServeStream(stream, manager, s.tokens, s.commitSessionWindowTerminal)
}
