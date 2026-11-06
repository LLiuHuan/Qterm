package tunnel

import (
	"context"
	"errors"
	"time"
)

const (
	keepaliveInterval = 15 * time.Second

	keepaliveReplyTimeout = 10 * time.Second

	keepaliveMaxFailures = 3

	keepaliveRequestName = "keepalive@openssh.com"
)

var (
	errKeepaliveTimeout  = errors.New("SSH keepalive probe timed out")
	errKeepaliveNotReady = errors.New("SSH keepalive connection is not ready")
)

func runKeepalive(
	ctx context.Context,
	interval time.Duration,
	timeout time.Duration,
	maxFailures int,
	probe func() error,
	onDead func(),
) {
	if ctx == nil || probe == nil || onDead == nil || interval <= 0 || timeout <= 0 || maxFailures <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		outcome := make(chan error, 1)
		go func() { outcome <- probe() }()
		var err error
		select {
		case err = <-outcome:
		case <-time.After(timeout):
			err = errKeepaliveTimeout
		case <-ctx.Done():
			return
		}
		if err == nil {
			failures = 0
			continue
		}
		failures++
		if failures >= maxFailures {
			onDead()
			return
		}
	}
}
