package tunnel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	processActionTimeout     = 6 * time.Second
	processActionOutputLimit = 4 * 1024
	processStartTimeLayout   = "2006-01-02 15:04:05"
	processLstartLayout      = "Mon Jan _2 15:04:05 2006"
)

type ProcessAction string

const (
	ProcessTerminate ProcessAction = "terminate"
	ProcessKill      ProcessAction = "kill"
)

var (
	ErrInvalidProcessPID        = errors.New("tunnel: invalid process pid")
	ErrMissingProcessStartTime  = errors.New("tunnel: process start time is required")
	ErrInvalidProcessStartTime  = errors.New("tunnel: invalid process start time")
	ErrInvalidProcessAction     = errors.New("tunnel: unsupported process action")
	ErrProcessNotFound          = errors.New("tunnel: process was not found")
	ErrProcessStartTimeMismatch = errors.New("tunnel: process start time changed")
	ErrProcessActionFailed      = errors.New("tunnel: process action failed")
)

func (c *Connection) RunProcessAction(
	ctx context.Context,
	pid int,
	expectedStartTime string,
	action ProcessAction,
) error {
	if ctx == nil {
		return errors.New("tunnel: process action context is nil")
	}
	if c == nil {
		return ErrRemoteOSUnsupported
	}
	if pid <= 1 {
		return ErrInvalidProcessPID
	}
	canonicalExpected, err := canonicalProcessStartTime(expectedStartTime)
	if err != nil {
		return err
	}

	var signal string
	switch action {
	case ProcessTerminate:
		signal = "TERM"
	case ProcessKill:
		signal = "KILL"
	default:
		return ErrInvalidProcessAction
	}

	actionCtx, cancel := context.WithTimeout(ctx, processActionTimeout)
	defer cancel()
	pidText := strconv.Itoa(pid)
	startOutput, err := c.runProbe(
		actionCtx,
		"LC_ALL=C ps -p "+pidText+" -o lstart= 2>/dev/null",
		processActionOutputLimit,
	)
	if err != nil {
		if contextErr := actionCtx.Err(); contextErr != nil {
			return contextErr
		}
		return ErrProcessNotFound
	}
	actualStartTime, ok := parseProcessStartTime(startOutput)
	if !ok {
		return ErrProcessNotFound
	}
	if actualStartTime != canonicalExpected {
		return ErrProcessStartTimeMismatch
	}

	if _, err := c.runProbe(
		actionCtx,
		"kill -"+signal+" "+pidText+" 2>/dev/null",
		processActionOutputLimit,
	); err != nil {
		if contextErr := actionCtx.Err(); contextErr != nil {
			return contextErr
		}
		return ErrProcessActionFailed
	}
	return nil
}

func canonicalProcessStartTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrMissingProcessStartTime
	}
	if started, err := time.Parse(processStartTimeLayout, trimmed); err == nil {
		return started.Format(processStartTimeLayout), nil
	}
	if started, err := time.Parse(processLstartLayout, trimmed); err == nil {
		return started.Format(processStartTimeLayout), nil
	}
	return "", ErrInvalidProcessStartTime
}

func parseProcessStartTime(output string) (string, bool) {
	fields := strings.Fields(output)
	if len(fields) < 5 {
		return "", false
	}
	for index := 0; index+5 <= len(fields); index++ {
		candidate := strings.Join(fields[index:index+5], " ")
		if started, err := time.Parse(processLstartLayout, candidate); err == nil {
			return started.Format(processStartTimeLayout), true
		}
	}
	return "", false
}
