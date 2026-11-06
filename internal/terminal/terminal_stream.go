package terminal

import (
	"bytes"
	sessionapp "changeme/internal/session"
	sshclient "changeme/internal/tunnel"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

var zmodemSignature = []byte{0x2a, 0x2a, 0x18, 0x42}

const (
	TerminalStreamName = "terminal"

	terminalOutputHighWatermark uint64 = 256 << 10
	terminalOutputLowWatermark  uint64 = 64 << 10

	terminalErrorProtocol  byte = 1
	terminalErrorBinding   byte = 2
	terminalErrorOpen      byte = 3
	terminalErrorTransport byte = 4
)

func terminalOpenReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sshclient.ErrTerminalHandshakeTimeout):

		return "远端未响应 shell 请求（超时）：请检查服务器的 PAM/会话限制或残留进程后重试"
	case errors.Is(err, sshclient.ErrInvalidTerminalSize):
		return "终端尺寸无效"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "打开终端已取消"
	default:
		return "无法打开终端：" + err.Error()
	}
}

func OpenErrorFrame(reason string) []byte {
	return terminalErrorFrame(terminalErrorOpen, reason)
}

type Stream interface {
	Context() context.Context
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
}

type SessionOwner interface {
	OpenTerminal(context.Context, string, int, int) (sessionapp.SessionTerminal, error)
	Close(context.Context, string) error
	CloseRemoteEnded(context.Context, string) error
}

type terminalPumpResult struct {
	protocol bool
	remote   bool
	detached bool
}

type terminalOutputWindow struct {
	mu          sync.Mutex
	outstanding uint64
	paused      bool
	wake        chan struct{}
}

func ServeStream(
	stream Stream,
	owner SessionOwner,
	tokens TokenVerifier,
	commitReady func(sessionID string, activate func() error) error,
) {
	if stream == nil || owner == nil || tokens == nil {
		return
	}
	defer stream.Close()

	frame, err := stream.Receive()
	if err != nil {
		return
	}
	op, payload, err := parseTerminalFrame(frame)
	if err != nil || op != terminalOpBind {
		_ = stream.Send(terminalErrorFrame(terminalErrorProtocol, "终端绑定帧无效"))
		return
	}
	bind, err := parseTerminalBind(payload)
	if err != nil {
		_ = stream.Send(terminalErrorFrame(terminalErrorProtocol, "终端绑定参数无效"))
		return
	}
	if !tokens.Consume(bind.sessionID, bind.token) {
		_ = stream.Send(terminalErrorFrame(terminalErrorBinding, "终端绑定令牌无效或已过期"))
		return
	}

	terminal, err := owner.OpenTerminal(stream.Context(), bind.sessionID, bind.columns, bind.rows)
	if err != nil {

		slog.Warn("terminal stream open failed",
			"event", "terminal.stream.open_failed",
			"session", bind.sessionID,
			"reason", terminalOpenReason(err),
		)
		_ = stream.Send(terminalErrorFrame(terminalErrorOpen, terminalOpenReason(err)))
		return
	}
	defer terminal.Close()

	ready, _ := terminalFrame(terminalOpReady, nil)
	if err := stream.Send(ready); err != nil {
		slog.Warn("terminal stream ready send failed",
			"event", "terminal.stream.ready_failed",
			"session", bind.sessionID,
		)
		return
	}
	slog.Debug("terminal stream ready",
		"event", "terminal.stream.ready",
		"session", bind.sessionID,
		"columns", bind.columns,
		"rows", bind.rows,
	)
	activate := func() error { return nil }
	if activator, ok := terminal.(interface{ Activate() error }); ok {
		activate = activator.Activate
	}
	if commitReady != nil {
		if err := commitReady(bind.sessionID, activate); err != nil {
			return
		}
	} else if err := activate(); err != nil {
		return
	}

	pumpCtx, cancel := context.WithCancel(stream.Context())
	window := newTerminalOutputWindow()
	results := make(chan terminalPumpResult, 3)
	go func() { results <- pumpTerminalOutput(pumpCtx, stream, terminal, window) }()
	go func() { results <- pumpTerminalInput(pumpCtx, stream, terminal, window) }()
	go func() {
		err := terminal.Wait()
		results <- terminalPumpResult{
			remote:   !errors.Is(err, sessionapp.ErrSessionTerminalDetached),
			detached: errors.Is(err, sessionapp.ErrSessionTerminalDetached),
		}
	}()

	result := <-results
	cancel()
	window.close()
	_ = terminal.Close()

	if stream.Context().Err() != nil {
		return
	}
	if result.protocol {
		_ = stream.Send(terminalErrorFrame(terminalErrorProtocol, "终端数据帧无效"))
		return
	}
	if result.detached {
		return
	}
	if result.remote {
		closeRemoteEndedSession(owner, bind.sessionID)
		closeFrame, _ := terminalFrame(terminalOpClose, []byte("终端已关闭"))
		_ = stream.Send(closeFrame)
		return
	}
	_ = stream.Send(terminalErrorFrame(terminalErrorTransport, "终端数据通道已关闭"))
}

type TokenVerifier interface {
	Consume(sessionID, token string) bool
}

func pumpTerminalOutput(
	ctx context.Context,
	stream Stream,
	terminal sessionapp.SessionTerminal,
	window *terminalOutputWindow,
) terminalPumpResult {
	buffer := make([]byte, terminalMaxDataSize)
	zmodemNotified := false
	for {
		if err := window.wait(ctx); err != nil {
			return terminalPumpResult{}
		}
		count, err := terminal.Read(buffer)
		if count > 0 {
			data := buffer[:count]

			if !zmodemNotified && bytes.Contains(data, zmodemSignature) {
				zmodemNotified = true

				notifyFrame, _ := terminalFrame(terminalOpZmodemDetected, []byte{0})
				_ = stream.Send(notifyFrame)
			}

			frame, frameErr := terminalFrame(terminalOpOutput, data)
			if frameErr != nil {
				return terminalPumpResult{protocol: true}
			}

			window.sent(uint64(count))
			if sendErr := stream.Send(frame); sendErr != nil {
				return terminalPumpResult{}
			}
		}
		if err != nil {
			if errors.Is(err, sessionapp.ErrSessionTerminalDetached) {
				return terminalPumpResult{detached: true}
			}
			return terminalPumpResult{remote: errors.Is(err, io.EOF)}
		}
	}
}

func pumpTerminalInput(
	ctx context.Context,
	stream Stream,
	terminal sessionapp.SessionTerminal,
	window *terminalOutputWindow,
) terminalPumpResult {
	for {
		if err := ctx.Err(); err != nil {
			return terminalPumpResult{}
		}
		frame, err := stream.Receive()
		if err != nil {
			return terminalPumpResult{}
		}
		op, payload, err := parseTerminalFrame(frame)
		if err != nil || op == terminalOpBind {
			return terminalPumpResult{protocol: true}
		}
		switch op {
		case terminalOpInput, terminalOpZmodem:
			if err := writeTerminalInput(terminal, payload); err != nil {
				return terminalPumpResult{}
			}
			if op == terminalOpZmodem {

				ack := make([]byte, 4)
				binary.BigEndian.PutUint32(ack, uint32(len(payload)))
				frame, _ := terminalFrame(terminalOpZmodemAck, ack)
				if err := stream.Send(frame); err != nil {
					return terminalPumpResult{}
				}
			}
		case terminalOpResize:
			columns, rows, err := parseTerminalResize(payload)
			if err != nil || terminal.Resize(columns, rows) != nil {
				return terminalPumpResult{protocol: true}
			}
		case terminalOpAck:
			consumed, err := parseTerminalAck(payload)
			if err != nil || !window.ack(uint64(consumed)) {
				return terminalPumpResult{protocol: true}
			}
		case terminalOpClose:
			return terminalPumpResult{detached: true}
		default:
			return terminalPumpResult{protocol: true}
		}
	}
}

func writeTerminalInput(terminal sessionapp.SessionTerminal, value []byte) error {
	for len(value) > 0 {
		count, err := terminal.Write(value)
		if count > 0 {
			value = value[count:]
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func newTerminalOutputWindow() *terminalOutputWindow {
	return &terminalOutputWindow{wake: make(chan struct{})}
}

func (w *terminalOutputWindow) sent(size uint64) {
	w.mu.Lock()
	w.outstanding += size
	w.mu.Unlock()
}

func (w *terminalOutputWindow) ack(size uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if size == 0 || size > w.outstanding {
		return false
	}
	w.outstanding -= size
	w.signalLocked()
	return true
}

func (w *terminalOutputWindow) wait(ctx context.Context) error {
	for {
		w.mu.Lock()
		if !w.paused && w.outstanding >= terminalOutputHighWatermark {
			w.paused = true
		}
		if w.paused && w.outstanding <= terminalOutputLowWatermark {
			w.paused = false
		}
		if !w.paused {
			w.mu.Unlock()
			return nil
		}
		wake := w.wake
		w.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *terminalOutputWindow) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.signalLocked()
}

func (w *terminalOutputWindow) signalLocked() {
	close(w.wake)
	w.wake = make(chan struct{})
}

func closeRemoteEndedSession(owner SessionOwner, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = owner.CloseRemoteEnded(ctx, sessionID)
}
