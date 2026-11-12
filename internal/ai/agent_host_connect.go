package ai

import (
	"context"
	"errors"
)

type HostSessionConnector interface {
	ConnectForHost(ctx context.Context, hostID string) (string, error)
}

var ErrConnectNeedsManual = errors.New("host requires a manual connection")
