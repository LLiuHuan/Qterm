package sftp

import "context"

func (s *Service) DirectoryFollow(ctx context.Context, sessionID string, configure bool) (string, error) {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return "", err
	}
	return manager.DirectoryFollow(ctx, sessionID, configure)
}
