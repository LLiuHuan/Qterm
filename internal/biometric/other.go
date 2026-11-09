//go:build !darwin || !cgo

package biometric

import "context"

func Available() bool { return false }

func Unlock(_ context.Context, _ string) error { return ErrUnavailable }
