package app

import (
	"context"
	"strings"

	"changeme/internal/errno"
)

func (s *Service) RememberHostPassword(ctx context.Context, hostID string, password string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return publicError(errno.New(errno.CodeInvalidArg, "host.id_required", ""))
	}
	if password == "" {
		return nil
	}
	if s == nil || s.hosts == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "host.not_ready", ""))
	}

	secret := []byte(password)
	defer clear(secret)
	if err := s.hosts.RememberHostPassword(ctx, hostID, secret); err != nil {
		return publicError(err)
	}
	return nil
}
