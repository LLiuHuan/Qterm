package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/remotedesktop"
	sshclient "changeme/internal/tunnel"
)

func rdpError(code errno.Code, messageKey string, err error) error {
	if err == nil {
		return errno.New(code, messageKey, "")
	}
	return errno.Wrap(code, messageKey, err)
}

func classifyRdpOpenError(err error) error {
	switch {

	case errors.Is(err, sshclient.ErrHostKeyUnknown), errors.Is(err, sshclient.ErrHostKeyChanged):
		return rdpError(errno.CodeConnFailed, "rdp.hostKeyRequired", err)
	case errors.Is(err, remotedesktop.ErrHostUnreachable):

		return rdpError(errno.CodeConnFailed, "rdp.hostUnreachable", err)
	case errors.Is(err, remotedesktop.ErrHelperMissing):
		return rdpError(errno.CodeConnFailed, "rdp.helperMissing", err)
	case errors.Is(err, remotedesktop.ErrUnknownEngine):
		return rdpError(errno.CodeInvalidArg, "rdp.unknownEngine", err)
	}
	return rdpError(errno.CodeConnFailed, "rdp.openFailed", err)
}

const defaultRdpEngine = "freerdp"

func rdpOptionsFor(address string, port int, username, domain, password string, width, height int, engine, certificateFingerprint string, clipboard remotedesktop.ClipboardSettings) remotedesktop.Options {
	resolved := strings.TrimSpace(engine)
	if resolved == "" {
		resolved = defaultRdpEngine
	}
	return remotedesktop.Options{
		Host:                   strings.TrimSpace(address),
		Port:                   port,
		Username:               username,
		Password:               password,
		Domain:                 domain,
		Width:                  width,
		Height:                 height,
		Engine:                 resolved,
		CertificateFingerprint: certificateFingerprint,

		Clipboard: remotedesktop.NormalizeClipboardSettings(clipboard),
	}
}

func (s *Service) RdpHelperEngines() ([]model.RdpHelperEngine, error) {
	availability := remotedesktop.HelperAvailabilityReport()
	result := make([]model.RdpHelperEngine, 0, len(availability))
	for _, item := range availability {
		result = append(result, model.RdpHelperEngine{
			Engine:  item.Engine,
			Ready:   item.Ready,
			Path:    item.Path,
			Message: item.Message,
		})
	}
	return result, nil
}

func rdpWindowURL(hostID, title string) string {
	params := url.Values{}
	params.Set("rdpWorkspace", "1")
	params.Set("rdpHost", strings.TrimSpace(hostID))
	if title = strings.TrimSpace(title); title != "" {
		params.Set("rdpTitle", title)
	}
	return "/?" + params.Encode()
}

func rdpSessionWindowURL(sessionID, hostID, title string) string {
	params := url.Values{}
	params.Set("rdpWorkspace", "1")
	params.Set("rdpSession", strings.TrimSpace(sessionID))
	if hostID = strings.TrimSpace(hostID); hostID != "" {

		params.Set("rdpHost", hostID)
	}
	if title = strings.TrimSpace(title); title != "" {
		params.Set("rdpTitle", title)
	}
	return "/?" + params.Encode()
}

func (s *Service) rdpManager() (*remotedesktop.Manager, error) {
	if s == nil || s.remoteDesktops == nil {
		return nil, rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	return s.remoteDesktops, nil
}

func (s *Service) RdpOpen(ctx context.Context, input model.RdpOpenInput) (model.RdpSession, error) {
	if _, err := s.rdpManager(); err != nil {
		return model.RdpSession{}, publicError(err)
	}

	fingerprint, err := s.trustedRdpFingerprint(input.Host, input.Port)
	if err != nil {
		return model.RdpSession{}, publicError(err)
	}

	session, err := s.openRdpOptions(ctx, rdpOptionsFor(input.Host, input.Port, input.Username, input.Domain,
		input.Password, input.Width, input.Height, input.Engine, fingerprint, remotedesktop.ClipboardSettings{}), nil)
	if err != nil {
		return model.RdpSession{}, publicError(classifyRdpOpenError(err))
	}
	stats := session.Stats()
	return model.RdpSession{
		ID:            session.ID(),
		Host:          input.Host,
		Engine:        stats.Engine,
		Width:         stats.Width,
		Height:        stats.Height,
		Connected:     stats.Connected,
		Error:         stats.Error,
		SecurityLabel: stats.SecurityLabel,
		Channels:      stats.Channels,
	}, nil
}

func (s *Service) RdpClose(id string) error {
	s.forgetRdpWindowSessionOwnership(id)
	s.releaseRdpRouteBridge(id)
	manager, err := s.rdpManager()
	if err != nil {
		return publicError(err)
	}
	return publicError(manager.Close(id))
}

func (s *Service) RdpListSessions() ([]model.RdpSession, error) {
	manager, err := s.rdpManager()
	if err != nil {
		return nil, publicError(err)
	}
	ids := manager.List()
	result := make([]model.RdpSession, 0, len(ids))
	for _, id := range ids {
		session, ok := manager.Get(id)
		if !ok {
			continue
		}
		stats := session.Stats()
		result = append(result, model.RdpSession{
			ID:            id,
			Width:         stats.Width,
			Height:        stats.Height,
			Connected:     stats.Connected,
			Error:         stats.Error,
			SecurityLabel: stats.SecurityLabel,
			Channels:      stats.Channels,
		})
	}
	return result, nil
}

func (s *Service) RdpFrame(id string) (model.RdpFrame, error) {
	manager, err := s.rdpManager()
	if err != nil {
		return model.RdpFrame{}, publicError(err)
	}
	session, ok := manager.Get(id)
	if !ok {
		return model.RdpFrame{}, publicError(rdpError(errno.CodeNotFound, "rdp.sessionMissing", nil))
	}
	frame, err := session.Frame()
	if err != nil {
		return model.RdpFrame{}, publicError(err)
	}
	return model.RdpFrame{
		SessionID:     id,
		Sequence:      frame.Sequence,
		Width:         frame.Width,
		Height:        frame.Height,
		Changed:       frame.Changed,
		PNG:           frame.PNG,
		CursorVisible: frame.CursorVisible,
		CursorX:       frame.CursorX,
		CursorY:       frame.CursorY,
		CursorWidth:   frame.CursorWidth,
		CursorHeight:  frame.CursorHeight,
		CursorPNG:     frame.CursorPNG,
	}, nil
}

func (s *Service) RdpResize(ctx context.Context, id string, width, height int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if width < 640 || width > 7680 || height < 480 || height > 4320 {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.invalidSize", nil))
	}
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	if err := session.Resize(width, height); err != nil {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.invalidSize", err))
	}
	return nil
}

func (s *Service) RdpStats(id string) (model.RdpStats, error) {
	manager, err := s.rdpManager()
	if err != nil {
		return model.RdpStats{}, publicError(err)
	}
	session, ok := manager.Get(id)
	if !ok {
		return model.RdpStats{}, publicError(rdpError(errno.CodeNotFound, "rdp.sessionMissing", nil))
	}
	stats := session.Stats()
	return model.RdpStats{
		SessionID:         id,
		Connected:         stats.Connected,
		Closed:            stats.Closed,
		Error:             stats.Error,
		Width:             stats.Width,
		Height:            stats.Height,
		Packets:           stats.Packets,
		ChannelPackets:    stats.ChannelPackets,
		BitmapUpdates:     stats.BitmapUpdates,
		OrderUpdates:      stats.OrderUpdates,
		PointerUpdates:    stats.PointerUpdates,
		OrdersApplied:     stats.OrdersApplied,
		ClipboardText:     stats.ClipboardText,
		ClipboardSequence: stats.ClipboardSequence,
		Engine:            stats.Engine,
		HelperStderr:      stats.HelperStderr,
		ResizeUnsupported: stats.ResizeUnsupported,

		CertificateState:       string(stats.CertificateState),
		CertificateFingerprint: stats.CertificateFingerprint,
		CertificateSubject:     stats.CertificateSubject,
		CertificateIssuer:      stats.CertificateIssuer,

		ServerFPS: stats.ServerFPS,
	}, nil
}

func (s *Service) RdpSendPointerMove(id string, x, y int) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	return publicError(session.SendPointerMove(x, y))
}

func (s *Service) RdpSendPointerButton(id string, x, y, button int, pressed bool) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	if button < 0 || button > 4 {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.invalidMouseButton", nil))
	}
	return publicError(session.SendPointerButton(x, y, remotedesktop.MouseButton(button), pressed))
}

func (s *Service) RdpSendWheel(id string, x, y, notches int, horizontal bool) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	return publicError(session.SendWheel(x, y, notches, horizontal))
}

func (s *Service) RdpSendKey(id string, scanCode int, pressed, extended bool) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	if scanCode < 0 || scanCode > 0xFFFF {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.invalidScanCode", nil))
	}
	return publicError(session.SendKey(uint16(scanCode), pressed, extended))
}

func (s *Service) RdpSendText(id string, text string) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}
	return publicError(session.SendText(text))
}

func (s *Service) RdpClipboardText(id string) (string, error) {
	session, err := s.rdpSession(id)
	if err != nil {
		return "", publicError(err)
	}
	return session.ClipboardText(), nil
}

const rdpClipboardRefreshTimeout = 500 * time.Millisecond

func (s *Service) RdpRefreshClipboard(id string) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}

	if !session.Clipboard().AllowsRemoteToLocal() {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.clipboardDirectionBlocked", nil))
	}

	if session.Stats().Closed {
		return publicError(errno.New(errno.CodeSessionClosed, "rdp.sessionMissing", ""))
	}
	before := session.ClipboardSequence()
	if err := session.RequestRemoteClipboard(); err != nil {
		return publicError(rdpError(errno.CodeConnFailed, "rdp.clipboardRefreshFailed", err))
	}

	session.WaitClipboardSequenceChange(before, rdpClipboardRefreshTimeout)
	return nil
}

func (s *Service) RdpSetClipboardText(id string, text string) error {
	session, err := s.rdpSession(id)
	if err != nil {
		return publicError(err)
	}

	if !session.Clipboard().AllowsLocalToRemote() {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.clipboardDirectionBlocked", nil))
	}
	return publicError(session.SetClipboardText(text))
}

func (s *Service) rdpSession(id string) (*remotedesktop.Session, error) {
	manager, err := s.rdpManager()
	if err != nil {
		return nil, err
	}
	session, ok := manager.Get(id)
	if !ok {
		return nil, rdpError(errno.CodeNotFound, "rdp.sessionMissing", nil)
	}
	return session, nil
}
