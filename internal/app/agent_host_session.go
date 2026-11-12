package app

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
)

func (executor sessionAgentExecutor) SessionForHost(ctx context.Context, hostID string) (string, error) {
	trimmed := strings.TrimSpace(hostID)
	if trimmed == "" || executor.sessions == nil {
		return "", ai.ErrNoOpenSession
	}
	manager, err := executor.sessions.Manager()
	if err != nil {
		return "", ai.ErrNoOpenSession
	}
	sessions, err := manager.List(ctx)
	if err != nil {
		return "", err
	}
	for _, session := range sessions {
		if session.HostID != trimmed {
			continue
		}

		if session.State != model.SessionStateConnected || session.Kind != model.SessionKindSSH {
			continue
		}
		if strings.TrimSpace(session.ID) == "" {
			continue
		}
		return session.ID, nil
	}
	return "", ai.ErrNoOpenSession
}

func (executor sessionAgentExecutor) ConnectForHost(ctx context.Context, hostID string) (string, error) {
	trimmed := strings.TrimSpace(hostID)
	if trimmed == "" {
		return "", errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}

	reused, reuseErr := executor.SessionForHost(ctx, trimmed)
	if reuseErr == nil {
		return reused, nil
	}

	if !errors.Is(reuseErr, ai.ErrNoOpenSession) {
		return "", reuseErr
	}

	if executor.hosts != nil {
		hosts, listErr := executor.hosts.ListHosts()
		if listErr == nil {
			found := false
			for _, host := range hosts {
				if host.ID != trimmed {
					continue
				}
				found = true
				if host.Protocol != model.ProtocolSSH {
					return "", errno.New(errno.CodeInvalidArg, "ai.agent.capability_unavailable", "")
				}
				break
			}
			if !found {
				return "", errno.New(errno.CodeNotFound, "session.not_found", "")
			}
		}
	}

	if executor.tempSessions == nil {

		return "", ai.ErrNoOpenSession
	}
	return executor.tempSessions.connect(ctx, trimmed)
}
