package security

import "errors"

var (
	ErrInvalidDEK = errors.New("invalid data encryption key")

	ErrInvalidCustomKey = errors.New("invalid custom encryption key")

	ErrKeyNotFound = errors.New("encryption key not found")

	ErrKeyExists = errors.New("encryption key already exists")

	ErrKeyringUnavailable = errors.New("system keyring unavailable")

	ErrInvalidEnvelope = errors.New("invalid encryption key envelope")

	ErrUnsupportedEnvelope = errors.New("unsupported encryption key envelope")

	ErrAuthentication = errors.New("encryption key authentication failed")

	ErrInvalidPath = errors.New("invalid security file path")

	ErrRecoveryPackage = errors.New("invalid recovery package")
)
