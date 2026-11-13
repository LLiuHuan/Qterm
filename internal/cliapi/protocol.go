package cliapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"changeme/internal/errno"
)

const (
	CodeCapabilityUnavailable errno.Code = "CAPABILITY_UNAVAILABLE"

	CodeAppLocked errno.Code = "APP_LOCKED"

	CodeBinaryFile errno.Code = "BINARY_FILE"

	CodeBodyTooLarge errno.Code = "BODY_TOO_LARGE"

	CodeUnauthorized errno.Code = "UNAUTHORIZED"

	CodeRouteNotFound errno.Code = "ROUTE_NOT_FOUND"
)

const (
	maxRequestBodyBytes = 1 << 20
	maxExecOutputBytes  = 512 << 10
	maxCommandBytes     = 8 << 10
	maxListEntries      = 300
	maxReadFileBytes    = 64 << 10
	maxWriteFileBytes   = 256 << 10

	requestTimeout = 60 * time.Second
	execTimeout    = 120 * time.Second
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StatusResponse struct {
	Version      string `json:"version"`
	PID          int    `json:"pid"`
	HostCount    int    `json:"hostCount"`
	SessionCount int    `json:"sessionCount"`
}

type HostView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
	Group    string   `json:"group,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type SessionView struct {
	ID     string `json:"id"`
	HostID string `json:"hostId"`
	Title  string `json:"title,omitempty"`
	Kind   string `json:"kind"`
	State  string `json:"state"`
}

type ExecResponse struct {
	ExitCode  int    `json:"exitCode"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

type SftpEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime,omitempty"`
}

type SftpListResponse struct {
	Path      string      `json:"path"`
	Entries   []SftpEntry `json:"entries"`
	Truncated bool        `json:"truncated"`
}

type SftpReadResponse struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Hash      string `json:"hash,omitempty"`
}

type SftpWriteResponse struct {
	Saved       bool   `json:"saved"`
	Conflict    bool   `json:"conflict"`
	CurrentHash string `json:"currentHash,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code errno.Code, message string) {
	if message == "" {
		message = string(code)
	}
	writeJSON(w, status, errorBody{Error: errorPayload{Code: string(code), Message: message}})
}

func writeAppError(w http.ResponseWriter, err error) {
	if appError := errno.Extract(err); appError != nil {
		writeError(w, statusForCode(appError.Code), appError.Code, appError.MessageKey)
		return
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusRequestTimeout, errno.CodeTimeout, "request timed out")
	case errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, errno.CodeCancelled, "request cancelled")
	default:
		writeError(w, http.StatusInternalServerError, errno.CodeUnknown, "internal error")
	}
}

func statusForCode(code errno.Code) int {
	switch code {
	case errno.CodeInvalidArg:
		return http.StatusBadRequest
	case errno.CodeNotFound:
		return http.StatusNotFound
	case errno.CodePermissionDenied:
		return http.StatusForbidden
	case errno.CodeSessionClosed, CodeCapabilityUnavailable, CodeBinaryFile:
		return http.StatusConflict
	case errno.CodeConnFailed, errno.CodeTimeout:
		return http.StatusBadGateway
	case errno.CodeCancelled:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}
