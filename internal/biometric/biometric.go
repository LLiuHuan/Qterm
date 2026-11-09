package biometric

import "errors"

var (
	ErrUnavailable = errors.New("biometric: unavailable")

	ErrNotVerified = errors.New("biometric: not verified")

	ErrTimeout = errors.New("biometric: timeout")
)
