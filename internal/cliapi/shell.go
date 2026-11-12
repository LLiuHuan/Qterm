package cliapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	CodeShellFrameTooLarge errno.Code = "SHELL_FRAME_TOO_LARGE"

	CodeShellInvalidFrame errno.Code = "SHELL_INVALID_FRAME"

	maxShellInputBytes = 64 << 10

	maxShellReadBytes = 256 << 10

	shellOutputChunk = 32 << 10

	shellWriteTimeout = 30 * time.Second

	shellDefaultColumns = 80
	shellDefaultRows    = 24
)

type ShellChannel interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(columns int, rows int) error
	Close() error

	ExitCode() int
}

type shellInputFrame struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type shellOutputFrame struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type shellExitFrame struct {
	Type string `json:"type"`
	Code int    `json:"code"`
}

type shellErrorFrame struct {
	Type       string `json:"type"`
	Code       string `json:"code"`
	MessageKey string `json:"messageKey"`
}

func shellSessionID(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/v1/sessions/")
	if !ok {
		return "", false
	}
	id, ok := strings.CutSuffix(rest, "/shell")
	if !ok || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request, sessionID string) {
	if s.deps.Shell == nil {
		writeError(w, http.StatusNotImplemented, CodeCapabilityUnavailable, "shell channel is not available")
		return
	}
	session, err := s.deps.Sessions.GetSession(r.Context(), sessionID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if session.State != model.SessionStateConnected {
		writeError(w, http.StatusConflict, errno.CodeSessionClosed, "session is not connected")
		return
	}
	columns, rows, ok := shellSizeFromQuery(r)
	if !ok {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "invalid terminal size")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	channel, err := s.deps.Shell(ctx, sessionID, columns, rows)
	if err != nil {
		writeAppError(w, err)
		return
	}
	defer func() { _ = channel.Close() }()

	connection, err := websocket.Accept(w, r, nil)
	if err != nil {

		return
	}
	defer func() {

		_ = connection.Close(websocket.StatusNormalClosure, "")
	}()
	connection.SetReadLimit(maxShellReadBytes)

	s.pumpShell(ctx, cancel, connection, channel)
}

func (s *Server) pumpShell(
	ctx context.Context,
	cancel context.CancelFunc,
	connection *websocket.Conn,
	channel ShellChannel,
) {
	type readResult struct {
		data []byte
		err  error
	}

	output := make(chan readResult, 8)
	inputs := make(chan shellErrorFrame, 1)

	var closeOnce sync.Once
	closeChannel := func() {
		closeOnce.Do(func() { _ = channel.Close() })
	}
	defer closeChannel()

	go func() {
		buffer := make([]byte, shellOutputChunk)
		for {
			count, err := channel.Read(buffer)
			if count > 0 {
				chunk := make([]byte, count)
				copy(chunk, buffer[:count])
				select {
				case output <- readResult{data: chunk}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				select {
				case output <- readResult{err: err}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	go func() {
		for {
			_, payload, err := connection.Read(ctx)
			if err != nil {

				cancel()
				closeChannel()
				return
			}
			var frame shellInputFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				inputs <- shellErrorFrame{Type: "error", Code: string(CodeShellInvalidFrame), MessageKey: "cli.shell.invalid_frame"}
				closeChannel()
				return
			}
			switch frame.Type {
			case "input":
				decoded, decodeErr := base64.StdEncoding.DecodeString(frame.Data)
				if decodeErr != nil {
					inputs <- shellErrorFrame{Type: "error", Code: string(CodeShellInvalidFrame), MessageKey: "cli.shell.invalid_frame"}
					closeChannel()
					return
				}
				if len(decoded) > maxShellInputBytes {
					inputs <- shellErrorFrame{Type: "error", Code: string(CodeShellFrameTooLarge), MessageKey: "cli.shell.frame_too_large"}
					closeChannel()
					return
				}
				if _, err := channel.Write(decoded); err != nil {
					closeChannel()
					return
				}
			case "resize":
				if err := channel.Resize(frame.Cols, frame.Rows); err != nil {
					inputs <- shellErrorFrame{Type: "error", Code: string(CodeShellInvalidFrame), MessageKey: "cli.shell.invalid_size"}
					closeChannel()
					return
				}
			default:
				inputs <- shellErrorFrame{Type: "error", Code: string(CodeShellInvalidFrame), MessageKey: "cli.shell.invalid_frame"}
				closeChannel()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case failure := <-inputs:
			_ = writeShellFrame(ctx, connection, failure)
			return
		case result := <-output:
			if result.err != nil {

				select {
				case failure := <-inputs:
					_ = writeShellFrame(ctx, connection, failure)
				default:
					_ = writeShellFrame(ctx, connection, shellExitFrame{Type: "exit", Code: channel.ExitCode()})
				}
				return
			}
			frame := shellOutputFrame{Type: "output", Data: base64.StdEncoding.EncodeToString(result.data)}
			if err := writeShellFrame(ctx, connection, frame); err != nil {
				return
			}
		}
	}
}

func writeShellFrame(ctx context.Context, connection *websocket.Conn, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, shellWriteTimeout)
	defer cancel()
	return connection.Write(writeCtx, websocket.MessageText, encoded)
}

func shellSizeFromQuery(r *http.Request) (int, int, bool) {
	columns, ok := parseShellDimension(r.URL.Query().Get("cols"), shellDefaultColumns)
	if !ok {
		return 0, 0, false
	}
	rows, ok := parseShellDimension(r.URL.Query().Get("rows"), shellDefaultRows)
	if !ok {
		return 0, 0, false
	}
	return columns, rows, true
}

func parseShellDimension(value string, fallback int) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 1000 {
		return 0, false
	}
	return parsed, true
}
