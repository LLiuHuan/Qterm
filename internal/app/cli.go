package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"changeme/internal/cliapi"
	"changeme/internal/host"
	"changeme/internal/model"
	"changeme/internal/remoteedit"
	"changeme/internal/session"
	"changeme/internal/sftp"
)

const cliHandshakeFileName = "cli.json"

type cliHosts struct{ hosts *host.HostService }

func (h cliHosts) ListHosts() ([]model.Host, error) { return h.hosts.ListHosts() }

type cliSessions struct{ sessions *session.Service }

func (s cliSessions) OpenSession(ctx context.Context, hostID string) (model.Session, error) {
	result, err := s.sessions.OpenSession(ctx, hostID)
	if err != nil {
		return model.Session{}, err
	}
	return result.Session, nil
}

func (s cliSessions) GetSession(ctx context.Context, id string) (model.Session, error) {
	return s.sessions.GetSession(ctx, id)
}

func (s cliSessions) ListSessions(ctx context.Context) ([]model.Session, error) {
	return s.sessions.ListSessions(ctx)
}

func (s cliSessions) CloseSession(ctx context.Context, id string) error {
	return s.sessions.CloseSession(ctx, id)
}

type cliExec struct{ sessions *session.Service }

func (e cliExec) RunBackgroundCommand(ctx context.Context, sessionID, command string) (cliapi.ExecResult, error) {
	result, err := e.sessions.RunAIBackgroundCommand(ctx, sessionID, command)
	if err != nil {
		return cliapi.ExecResult{}, err
	}
	return cliapi.ExecResult{Output: result.Output, ExitCode: result.ExitCode}, nil
}

type cliShell struct{ sessions *session.Service }

func (c cliShell) open(
	ctx context.Context,
	sessionID string,
	columns int,
	rows int,
) (cliapi.ShellChannel, error) {
	terminal, err := c.sessions.OpenAuxiliaryTerminal(ctx, sessionID, columns, rows)
	if err != nil {
		return nil, err
	}
	return terminal, nil
}

type cliFiles struct {
	sftp        *sftp.Service
	remoteEdits *remoteedit.Manager
}

func (f cliFiles) ListRemoteDirectory(ctx context.Context, sessionID, path string) ([]model.RemoteFileEntry, error) {
	return f.sftp.ListRemoteDirectory(ctx, sessionID, path)
}

func (f cliFiles) ReadRemoteText(ctx context.Context, sessionID, path string) (model.RemoteFileContent, error) {
	return f.remoteEdits.ReadText(ctx, sessionID, path)
}

func (f cliFiles) WriteRemoteText(ctx context.Context, sessionID, path, contentBase64, expectedHash string) (model.RemoteFileWriteResult, error) {
	return f.remoteEdits.WriteText(ctx, sessionID, path, contentBase64, expectedHash)
}

func (s *Service) startCLIServer(ctx context.Context) error {
	if s.sessions == nil || s.sftp == nil || s.remoteEdits == nil || s.hosts == nil {
		return nil
	}
	server, err := cliapi.NewServer(cliapi.Deps{
		Hosts:    cliHosts{hosts: s.hosts},
		Sessions: cliSessions{sessions: s.sessions},
		Exec:     cliExec{sessions: s.sessions},
		Files: cliFiles{
			sftp:        s.sftp,
			remoteEdits: s.remoteEdits,
		},
		Shell:   cliShell{sessions: s.sessions}.open,
		Version: "qterm",

		Locked: func() bool { return s.appLocked.Load() },
	})
	if err != nil {
		return err
	}
	port, err := server.Start()
	if err != nil {
		_ = server.Close()
		return err
	}
	path := s.cliHandshakePath()
	if err := cliapi.WriteHandshake(path, cliapi.Handshake{
		Version: cliapi.HandshakeVersion,
		Port:    port,
		Token:   server.Token(),
		PID:     os.Getpid(),
	}); err != nil {
		_ = server.Close()
		return err
	}
	s.cliServer = server
	s.cliHandshake = path
	return nil
}

func (s *Service) stopCLIServer() {
	if s == nil {
		return
	}
	if s.cliHandshake != "" {
		cliapi.RemoveHandshake(s.cliHandshake)
	}
	if s.cliServer != nil {
		_ = s.cliServer.Close()
	}
}

func (s *Service) cliHandshakePath() string {
	root := ""
	if s.security != nil {
		root = s.security.Root()
	}
	if root == "" {
		home, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(home, "Qterm")
	}
	return filepath.Join(root, cliHandshakeFileName)
}

func logCLIError(err error) {
	if err == nil {
		return
	}
	slog.Warn("cli server unavailable", "error", strings.TrimSpace(err.Error()))
}
