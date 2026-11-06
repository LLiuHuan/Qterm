package tunnel

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

const maxAICommandOutput = 128 * 1024

type AICommandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
}

type boundedAIOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	truncated bool
}

func (w *boundedAIOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := maxAICommandOutput - w.buffer.Len()
	if remaining > 0 {
		if len(data) < remaining {
			remaining = len(data)
		}
		_, _ = w.buffer.Write(data[:remaining])
	}
	if len(data) > remaining {
		w.truncated = true
	}

	return len(data), nil
}

func (w *boundedAIOutput) result(exitCode int) AICommandResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	output := w.buffer.String()
	if w.truncated {
		output += "\n… [output truncated]"
	}
	return AICommandResult{Output: output, ExitCode: exitCode}
}

func (c *Connection) RunAICommand(ctx context.Context, command string) (AICommandResult, error) {
	if c == nil || c.client == nil {
		return AICommandResult{}, errors.New("SSH connection is not ready")
	}
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 8192 {
		return AICommandResult{}, errors.New("invalid AI command length")
	}
	for _, char := range command {
		if char < 0x20 || char == 0x7f {
			return AICommandResult{}, errors.New("AI command contains control characters")
		}
	}
	session, err := openCommandChannel(ctx, c.client)
	if err != nil {
		return AICommandResult{}, err
	}
	defer session.Close()
	output := &boundedAIOutput{}
	session.Stdout = output
	session.Stderr = output

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			_ = session.Close()
		case <-stop:
		}
	}()
	runErr := session.Run(command)
	close(stop)
	<-done
	if err := ctx.Err(); err != nil {
		return AICommandResult{}, err
	}
	if runErr == nil {
		return output.result(0), nil
	}
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		return output.result(exitErr.ExitStatus()), nil
	}
	return AICommandResult{}, runErr
}
