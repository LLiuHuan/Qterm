package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/term"
)

const (
	maxShellInputFrameBytes = 64 << 10
	shellFrameWriteTimeout  = 30 * time.Second
	shellDefaultColumns     = 80
	shellDefaultRows        = 24
)

type shellInputFrame struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type shellServerFrame struct {
	Type       string          `json:"type"`
	Data       string          `json:"data,omitempty"`
	Code       json.RawMessage `json:"code,omitempty"`
	MessageKey string          `json:"messageKey,omitempty"`
}

func (c *client) cmdShell(args []string) error {
	if len(args) == 0 {
		return usageError("shell: expects <sessionID> [--cols N] [--rows N]")
	}
	sessionID := args[0]
	fs := flag.NewFlagSet("shell", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	columns := fs.Int("cols", 0, "initial PTY columns (default: local terminal width)")
	rows := fs.Int("rows", 0, "initial PTY rows (default: local terminal height)")
	if err := fs.Parse(args[1:]); err != nil {
		return usageError(fmt.Sprintf("shell: %v", err))
	}
	if fs.NArg() != 0 {
		return usageError("shell: unexpected arguments")
	}

	stdin := int(os.Stdin.Fd())
	if !term.IsTerminal(stdin) {
		return usageError("shell: requires an interactive terminal on stdin")
	}
	columnsValue, rowsValue := resolveShellSize(*columns, *rows)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connection, err := c.dialShell(ctx, sessionID, columnsValue, rowsValue)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close(websocket.StatusNormalClosure, "") }()

	previous, err := term.MakeRaw(stdin)
	if err != nil {
		return fmt.Errorf("switch terminal to raw mode: %w", err)
	}
	defer func() {

		_ = term.Restore(stdin, previous)
	}()

	return c.runShellSession(ctx, connection, stdin, columnsValue, rowsValue)
}

func resolveShellSize(columns int, rows int) (int, int) {
	if columns > 0 && rows > 0 {
		return columns, rows
	}
	localColumns, localRows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || localColumns <= 0 || localRows <= 0 {
		localColumns, localRows = shellDefaultColumns, shellDefaultRows
	}
	if columns <= 0 {
		columns = localColumns
	}
	if rows <= 0 {
		rows = localRows
	}
	return columns, rows
}

func (c *client) dialShell(ctx context.Context, sessionID string, columns, rows int) (*websocket.Conn, error) {
	query := url.Values{
		"cols": {strconv.Itoa(columns)},
		"rows": {strconv.Itoa(rows)},
	}
	target := strings.Replace(c.baseURL, "http://", "ws://", 1) +
		"/v1/sessions/" + url.PathEscape(sessionID) + "/shell?" + query.Encode()
	connection, response, err := websocket.Dial(ctx, target, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + c.token}},
	})
	if err == nil {
		return connection, nil
	}

	if response != nil {
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode == http.StatusUnauthorized {
			return nil, errAuth
		}
		if body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseSize)); readErr == nil {
			if apiErr := decodeAPIError(response.StatusCode, body); apiErr != nil {
				return nil, apiErr
			}
		}
	}
	return nil, fmt.Errorf("%w: %v", errConnect, err)
}

func (c *client) runShellSession(
	parent context.Context,
	connection *websocket.Conn,
	stdin int,
	columns int,
	rows int,
) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var writeMu sync.Mutex
	writeFrame := func(frame shellInputFrame) error {
		payload, err := json.Marshal(frame)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		writeCtx, writeCancel := context.WithTimeout(ctx, shellFrameWriteTimeout)
		defer writeCancel()
		return connection.Write(writeCtx, websocket.MessageText, payload)
	}

	input := os.NewFile(uintptr(stdin), "stdin")
	go func() {
		buffer := make([]byte, 32<<10)
		for {
			count, err := input.Read(buffer)
			if count > 0 {
				chunk := buffer[:count]
				if len(chunk) > maxShellInputFrameBytes {
					chunk = chunk[:maxShellInputFrameBytes]
				}
				if writeErr := writeFrame(shellInputFrame{
					Type: "input",
					Data: base64.StdEncoding.EncodeToString(chunk),
				}); writeErr != nil {
					cancel()
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	winch := make(chan os.Signal, 1)
	notifyShellResize(winch)
	defer stopShellResize(winch)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-winch:
				nextColumns, nextRows, sizeErr := term.GetSize(stdin)
				if sizeErr != nil || nextColumns <= 0 || nextRows <= 0 {
					continue
				}
				if nextColumns == columns && nextRows == rows {
					continue
				}
				columns, rows = nextColumns, nextRows
				if writeErr := writeFrame(shellInputFrame{Type: "resize", Cols: columns, Rows: rows}); writeErr != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		_, payload, err := connection.Read(ctx)
		if err != nil {

			if websocket.CloseStatus(err) == websocket.StatusNormalClosure || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("%w: %v", errConnect, err)
		}
		var frame shellServerFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			continue
		}
		switch frame.Type {
		case "output":
			decoded, decodeErr := base64.StdEncoding.DecodeString(frame.Data)
			if decodeErr != nil {
				continue
			}
			_, _ = c.stdout.Write(decoded)
		case "exit":
			var code int
			if len(frame.Code) > 0 {
				_ = json.Unmarshal(frame.Code, &code)
			}
			if code != 0 {
				fmt.Fprintf(c.stderr, "qterm: remote exit code %d\n", code)
			}
			return nil
		case "error":
			var code string
			if len(frame.Code) > 0 {
				_ = json.Unmarshal(frame.Code, &code)
			}
			if code == "" {
				code = "SHELL_FAILED"
			}
			return &apiError{status: http.StatusBadGateway, code: code, message: frame.MessageKey}
		}
	}
}

func decodeAPIError(status int, body []byte) error {
	var parsed struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.Error.Code == "" {
		return nil
	}
	return &apiError{status: status, code: parsed.Error.Code, message: parsed.Error.Message}
}
