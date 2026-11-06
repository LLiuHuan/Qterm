package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	defaultTelnetPort = 23

	telnetProtocolName = "telnet"

	maxTelnetFieldSize = 1024

	telnetDefaultColumns = 80
	telnetDefaultRows    = 24

	telnetReadBufferBytes = 32 << 10
)

var (
	ErrTelnetClosed = errors.New("telnet terminal is closed")

	ErrInvalidTelnetTarget = errors.New("invalid telnet target")
)

type TelnetTarget struct {
	Address string
	Port    int

	TerminalType string

	Timeout time.Duration

	Columns int
	Rows    int
}

type TelnetTerminal struct {
	conn         net.Conn
	parser       telnetParser
	terminalType string

	writeMu sync.Mutex

	readMu  sync.Mutex
	readBuf []byte
	pending []byte

	negoMu     sync.Mutex
	negotiator *telnetNegotiator

	sizeMu  sync.Mutex
	columns int
	rows    int

	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error

	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error
}

func DialTelnet(ctx context.Context, target TelnetTarget) (*TelnetTerminal, error) {
	if ctx == nil {
		return nil, errors.New("telnet context is nil")
	}
	address, err := normalizeTelnetTarget(target)
	if err != nil {
		return nil, &ConnectError{Stage: StageValidation, Err: err, Protocol: telnetProtocolName}
	}
	terminalType := strings.TrimSpace(target.TerminalType)
	if terminalType == "" {
		terminalType = defaultTerminalType
	}
	columns, rows := normalizeTelnetSize(target.Columns, target.Rows)

	dialCtx := ctx
	if target.Timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, target.Timeout)
		defer cancel()
	}
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, telnetConnectError(dialCtx, StageTCP, err)
	}

	terminal := &TelnetTerminal{
		conn:         conn,
		terminalType: terminalType,
		readBuf:      make([]byte, telnetReadBufferBytes),
		negotiator:   newTelnetNegotiator(),
		columns:      columns,
		rows:         rows,
		waitDone:     make(chan struct{}),
	}
	if err := terminal.startNegotiation(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return terminal, nil
}

func (t *TelnetTerminal) startNegotiation() error {
	var frames []byte
	t.negoMu.Lock()
	for _, option := range []byte{telnetOptSuppressGA, telnetOptTerminalType, telnetOptNAWS} {
		for _, command := range t.negotiator.requestLocalEnable(option) {
			frames = append(frames, telnetCommandFrame(command.verb, command.option)...)
		}
	}
	for _, command := range t.negotiator.requestRemoteEnable(telnetOptSuppressGA) {
		frames = append(frames, telnetCommandFrame(command.verb, command.option)...)
	}
	t.negoMu.Unlock()
	if len(frames) == 0 {
		return nil
	}
	return t.writeFrames(frames)
}

func (t *TelnetTerminal) Read(buffer []byte) (int, error) {
	if t == nil {
		return 0, ErrTelnetClosed
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	t.readMu.Lock()
	defer t.readMu.Unlock()
	for {
		if len(t.pending) > 0 {
			count := copy(buffer, t.pending)
			t.pending = t.pending[count:]
			return count, nil
		}
		count, readErr := t.conn.Read(t.readBuf)
		if count > 0 {
			var commands []telnetCommand
			var subs []telnetSubnegotiation
			t.pending, commands, subs = t.parser.feed(t.readBuf[:count], t.pending[:0])
			if handleErr := t.handleInbound(commands, subs); handleErr != nil {
				t.finish(handleErr)
				return 0, handleErr
			}
			if len(t.pending) > 0 {

				continue
			}
		}
		if readErr != nil {
			t.finish(readErr)
			if t.Closed() {

				return 0, io.EOF
			}
			return 0, readErr
		}
	}
}

func (t *TelnetTerminal) Write(value []byte) (int, error) {
	if t == nil || t.Closed() {
		return 0, ErrTelnetClosed
	}
	if len(value) == 0 {
		return 0, nil
	}
	frame := appendTelnetEscaped(make([]byte, 0, len(value)+8), value)
	if err := t.writeFrames(frame); err != nil {
		return 0, err
	}
	return len(value), nil
}

func (t *TelnetTerminal) Resize(columns int, rows int) error {
	if err := validateTerminalSize(columns, rows); err != nil {
		return err
	}
	if t == nil || t.Closed() {
		return ErrTelnetClosed
	}
	t.sizeMu.Lock()
	t.columns = columns
	t.rows = rows
	t.sizeMu.Unlock()
	return t.sendNAWSIfAgreed()
}

func (t *TelnetTerminal) Wait() error {
	if t == nil {
		return nil
	}
	<-t.waitDone
	return t.waitErr
}

func (t *TelnetTerminal) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.closeErr = t.conn.Close()

		t.finish(nil)
	})
	return t.closeErr
}

func (t *TelnetTerminal) Closed() bool {
	return t == nil || t.closed.Load()
}

func (t *TelnetTerminal) NegotiationSummary() string {
	if t == nil {
		return ""
	}
	t.negoMu.Lock()
	defer t.negoMu.Unlock()
	return telnetNegotiationSummary(t.negotiator)
}

func (t *TelnetTerminal) handleInbound(commands []telnetCommand, subs []telnetSubnegotiation) error {
	var frames []byte
	t.negoMu.Lock()
	for _, command := range commands {
		for _, reply := range t.negotiator.receive(command.verb, command.option) {
			frames = append(frames, telnetCommandFrame(reply.verb, reply.option)...)
		}

		if command.verb == telnetDO && command.option == telnetOptNAWS &&
			t.negotiator.localEnabled(telnetOptNAWS) {
			frames = append(frames, telnetSubnegotiationFrame(telnetOptNAWS, t.nawsPayload())...)
		}
	}
	for _, sub := range subs {
		if sub.option != telnetOptTerminalType {
			continue
		}

		if len(sub.payload) != 1 || sub.payload[0] != telnetTerminalTypeSend {
			continue
		}
		frames = append(frames, telnetSubnegotiationFrame(
			telnetOptTerminalType,
			telnetTerminalTypePayload(t.terminalType),
		)...)
	}
	t.negoMu.Unlock()
	if len(frames) == 0 {
		return nil
	}
	return t.writeFrames(frames)
}

func (t *TelnetTerminal) sendNAWSIfAgreed() error {
	t.negoMu.Lock()
	enabled := t.negotiator.localEnabled(telnetOptNAWS)
	t.negoMu.Unlock()
	if !enabled {
		return nil
	}
	return t.writeFrames(telnetSubnegotiationFrame(telnetOptNAWS, t.nawsPayload()))
}

func (t *TelnetTerminal) nawsPayload() []byte {
	t.sizeMu.Lock()
	columns, rows := t.columns, t.rows
	t.sizeMu.Unlock()
	return telnetNAWSPayload(columns, rows)
}

func (t *TelnetTerminal) writeFrames(frames []byte) error {
	if t == nil {
		return ErrTelnetClosed
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if t.Closed() {
		return ErrTelnetClosed
	}
	if _, err := t.conn.Write(frames); err != nil {
		return fmt.Errorf("write telnet frames: %w", err)
	}
	return nil
}

func (t *TelnetTerminal) finish(err error) {
	if t == nil {
		return
	}
	t.waitOnce.Do(func() {
		if t.Closed() {

			err = nil
		}
		t.waitErr = err
		close(t.waitDone)
	})
}

func telnetConnectError(ctx context.Context, stage Stage, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		err = contextErr
	}
	return &ConnectError{Stage: stage, Err: err, Protocol: telnetProtocolName}
}

func normalizeTelnetTarget(target TelnetTarget) (string, error) {
	address := strings.TrimSpace(target.Address)
	if address == "" || len(address) > maxTelnetFieldSize ||
		!utf8.ValidString(address) || strings.IndexByte(address, 0) >= 0 {
		return "", ErrInvalidTelnetTarget
	}
	port := target.Port
	if port == 0 {
		port = defaultTelnetPort
	}
	if port < 1 || port > 65535 {
		return "", ErrInvalidTelnetTarget
	}
	return net.JoinHostPort(address, strconv.Itoa(port)), nil
}

func normalizeTelnetSize(columns int, rows int) (int, int) {
	if validateTerminalSize(columns, rows) != nil {
		return telnetDefaultColumns, telnetDefaultRows
	}
	return columns, rows
}
