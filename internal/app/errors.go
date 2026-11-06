package app

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"encoding/json"
	"errors"
)

func publicError(err error) error {
	if errno.Extract(err) == nil && (errors.Is(err, model.ErrInvalidHost) || errors.Is(err, model.ErrInvalidGroup) || errors.Is(err, model.ErrInvalidCredential) || errors.Is(err, model.ErrInvalidCredentialPromptRule) || errors.Is(err, model.ErrInvalidSession) || errors.Is(err, model.ErrInvalidSFTPTransfer)) {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if errors.Is(err, model.ErrCredentialVaultLocked) {
		return errno.New(errno.CodePermissionDenied, "credential.locked", "")
	}
	return errno.Public(err)
}

func marshalError(err error) []byte {
	data, marshalErr := json.Marshal(publicError(err))
	if marshalErr != nil {
		return []byte(`{"code":"UNKNOWN","messageKey":"error.unknown"}`)
	}
	return data
}
