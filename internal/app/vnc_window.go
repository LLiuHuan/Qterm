package app

import (
	"context"
	"net/url"
	"strings"

	"changeme/internal/errno"
)

func vncWindowURL(hostID, title string) string {
	params := url.Values{}
	params.Set("vncWorkspace", "1")
	params.Set("vncHost", strings.TrimSpace(hostID))
	if title = strings.TrimSpace(title); title != "" {
		params.Set("vncTitle", title)
	}
	return "/?" + params.Encode()
}

func (s *Service) OpenVncWindow(ctx context.Context, hostID, title string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return vncError(errno.CodeInvalidArg, "vnc.hostRequired", nil)
	}
	if s == nil || s.openWindow == nil {
		return vncError(errno.CodeConnFailed, "vnc.serviceNotReady", nil)
	}
	s.openWindow(ctx, vncWindowURL(hostID, title), nil)
	return nil
}
