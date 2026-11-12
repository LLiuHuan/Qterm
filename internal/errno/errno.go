package errno

import (
	"context"
	"errors"
)

type Code string

const (
	CodeUnknown          Code = "UNKNOWN"
	CodeInvalidArg       Code = "INVALID_ARG"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConnFailed       Code = "CONN_FAILED"
	CodeSessionClosed    Code = "SESSION_CLOSED"
	CodeStoreFailed      Code = "STORE_FAILED"
	CodeCancelled        Code = "CANCELLED"
	CodeTimeout          Code = "TIMEOUT"
	CodePermissionDenied Code = "PERMISSION_DENIED"

	CodeUnsupported Code = "UNSUPPORTED"
)

type AppError struct {
	Code       Code   `json:"code"`
	MessageKey string `json:"messageKey"`
	SafeDetail string `json:"safeDetail,omitempty"`
	cause      error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code) + ": " + e.MessageKey
}

func New(code Code, messageKey, safeDetail string) *AppError {
	return &AppError{Code: code, MessageKey: messageKey, SafeDetail: safeDetail}
}

func Wrap(code Code, messageKey string, err error) error {
	if err == nil {
		return nil
	}
	if Extract(err) != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &AppError{Code: code, MessageKey: messageKey, cause: err}
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func Extract(err error) *AppError {
	var value *AppError
	if errors.As(err, &value) {
		return value
	}
	return nil
}

func Public(err error) error {
	if err == nil {
		return nil
	}
	if value := Extract(err); value != nil {
		return New(value.Code, value.MessageKey, value.SafeDetail)
	}
	if errors.Is(err, context.Canceled) {
		return New(CodeCancelled, "error.cancelled", "")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return New(CodeTimeout, "error.timeout", "")
	}
	return New(CodeUnknown, "error.unknown", "")
}
