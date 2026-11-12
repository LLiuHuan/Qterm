package app

import (
	"strconv"

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
	window := stream.Window()
	if window == nil {
		_ = stream.Send(terminal.OpenErrorFrame("终端窗口已关闭"))
		_ = stream.Close()
		return
	}
	s.mu.Lock()
	control := s.terminalControl
	s.mu.Unlock()
	if control == nil {
		_ = stream.Send(terminal.OpenErrorFrame("终端服务尚未就绪"))
		_ = stream.Close()
		return
	}
	terminal.ServeStreamWithInputRouter(
		stream,
		manager,
		s.tokens,
		s.commitSessionWindowTerminal,
		control,
		strconv.FormatUint(uint64(window.ID()), 10),
	)
}
