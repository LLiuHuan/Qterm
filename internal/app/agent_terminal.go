package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/session"
)

const (
	terminalCaptureIdle    = 600 * time.Millisecond
	terminalCaptureMaxByte = 128 << 10
)

type terminalAgentExecutor struct {
	sessions *session.Service
}

func (executor terminalAgentExecutor) Execute(ctx context.Context, request ai.ExecRequest) (ai.StepResult, error) {
	command := normalizeTerminalCommand(request.Command)
	if command == "" {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.invalid_arg", "terminal execution requires a single-line command")
	}
	if executor.sessions == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	manager, err := executor.sessions.Manager()
	if err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeConnFailed, "ai.agent.execution_failed", err)
	}
	attachment, err := manager.OpenTerminal(ctx, request.SessionID, 120, 40)
	if err != nil {
		if errors.Is(err, session.ErrSessionTerminalUnsupported) || errors.Is(err, session.ErrSessionTerminalAttached) {
			return ai.StepResult{}, ai.ErrCapabilityUnavailable
		}
		return ai.StepResult{}, errno.Wrap(errno.CodeConnFailed, "ai.agent.execution_failed", err)
	}
	defer func() { _ = attachment.Close() }()

	if err := manager.WriteTerminalInput(ctx, request.SessionID, []byte(command+"\n")); err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeConnFailed, "ai.agent.execution_failed", err)
	}
	output, err := readTerminalOutput(ctx, attachment)
	if err != nil {
		return ai.StepResult{}, err
	}
	cleaned := trimTerminalEcho(stripTerminalANSI(output), command)

	return ai.StepResult{Observation: cleaned}, nil
}

func normalizeTerminalCommand(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n") {
		return ""
	}
	cleaned := strings.Map(func(char rune) rune {
		if char == '\t' {
			return char
		}
		if unicode.IsControl(char) {
			return -1
		}
		return char
	}, trimmed)
	return strings.TrimSpace(cleaned)
}

func stripTerminalANSI(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); {
		if value[index] == 0x1b && index+1 < len(value) && value[index+1] == '[' {
			index += 2
			for index < len(value) {
				char := value[index]
				index++
				if char >= 0x40 && char <= 0x7e {
					break
				}
			}
			continue
		}
		if value[index] == '\b' {
			index++
			continue
		}
		builder.WriteByte(value[index])
		index++
	}
	return builder.String()
}

func trimTerminalEcho(output, command string) string {
	trimmed := strings.TrimLeft(output, "\r\n")
	lines := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.Contains(lines[0], command) {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func readTerminalOutput(ctx context.Context, attachment session.SessionTerminal) (string, error) {
	type chunk struct {
		data []byte
		err  error
	}
	chunks := make(chan chunk, 8)
	go func() {
		buffer := make([]byte, 4096)
		for {
			count, err := attachment.Read(buffer)
			if count > 0 {
				copied := append([]byte(nil), buffer[:count]...)
				select {
				case chunks <- chunk{data: copied}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				select {
				case chunks <- chunk{err: err}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	var builder strings.Builder
	idle := time.NewTimer(terminalCaptureIdle)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return builder.String(), ctx.Err()
		case <-idle.C:
			return builder.String(), nil
		case item := <-chunks:
			if item.err != nil {
				if errors.Is(item.err, io.EOF) {
					return builder.String(), nil
				}

				return builder.String(), nil
			}
			if builder.Len()+len(item.data) > terminalCaptureMaxByte {
				remaining := terminalCaptureMaxByte - builder.Len()
				if remaining > 0 {
					builder.Write(item.data[:remaining])
				}
				return builder.String(), nil
			}
			builder.Write(item.data)
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(terminalCaptureIdle)
		}
	}
}
