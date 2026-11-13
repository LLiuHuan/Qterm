package cliapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const CodeMethodNotAllowed errno.Code = "METHOD_NOT_ALLOWED"

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method
	switch {
	case path == "/v1/status":
		if method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleStatus(w, r)
	case path == "/v1/hosts":
		if method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleHosts(w, r)
	case path == "/v1/sessions":
		switch method {
		case http.MethodGet:
			s.handleListSessions(w, r)
		case http.MethodPost:
			s.handleOpenSession(w, r)
		default:
			methodNotAllowed(w)
		}
	case strings.HasPrefix(path, "/v1/sessions/"):

		if sessionID, ok := shellSessionID(path); ok {
			if method != http.MethodGet {
				methodNotAllowed(w)
				return
			}
			s.handleShell(w, r, sessionID)
			return
		}
		if method != http.MethodDelete {
			methodNotAllowed(w)
			return
		}
		s.handleCloseSession(w, r, strings.TrimPrefix(path, "/v1/sessions/"))
	case path == "/v1/exec":
		if method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.handleExec(w, r)
	case path == "/v1/sftp/list":
		if method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleSftpList(w, r)
	case path == "/v1/sftp/read":
		if method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleSftpRead(w, r)
	case path == "/v1/sftp/write":
		if method != http.MethodPut {
			methodNotAllowed(w)
			return
		}
		s.handleSftpWrite(w, r)
	default:
		writeError(w, http.StatusNotFound, CodeRouteNotFound, "unknown route")
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	hosts, err := s.deps.Hosts.ListHosts()
	if err != nil {
		writeAppError(w, err)
		return
	}
	sessions, err := s.deps.Sessions.ListSessions(ctx)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, StatusResponse{
		Version:      s.deps.Version,
		PID:          s.deps.PID,
		HostCount:    len(hosts),
		SessionCount: len(sessions),
	})
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.deps.Hosts.ListHosts()
	if err != nil {
		writeAppError(w, err)
		return
	}
	views := make([]HostView, 0, len(hosts))
	for _, host := range hosts {
		views = append(views, projectHost(host))
	}
	writeJSON(w, http.StatusOK, views)
}

func projectHost(host model.Host) HostView {
	return HostView{
		ID:       host.ID,
		Name:     host.Name,
		Address:  host.Address,
		Port:     host.Port,
		Protocol: host.Protocol,
		Group:    host.Group,
		Tags:     append([]string(nil), host.Tags...),
	}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	sessions, err := s.deps.Sessions.ListSessions(ctx)
	if err != nil {
		writeAppError(w, err)
		return
	}
	titles := s.hostNames()
	views := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		views = append(views, projectSession(session, titles))
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleOpenSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		HostID string `json:"hostId"`
	}
	if !decodeJSONBody(w, r, &payload) {
		return
	}
	hostID := strings.TrimSpace(payload.HostID)
	if hostID == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "hostId is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	session, err := s.deps.Sessions.OpenSession(ctx, hostID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectSession(session, s.hostNames()))
}

func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		writeError(w, http.StatusNotFound, CodeRouteNotFound, "session id is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	if err := s.deps.Sessions.CloseSession(ctx, sessionID); err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID string `json:"sessionId"`
		Command   string `json:"command"`
	}
	if !decodeJSONBody(w, r, &payload) {
		return
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "sessionId is required")
		return
	}
	command := payload.Command

	if strings.TrimSpace(command) == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "command is required")
		return
	}
	if strings.IndexByte(command, 0) >= 0 {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "command contains NUL")
		return
	}
	if len(command) > maxCommandBytes {
		writeError(w, http.StatusRequestEntityTooLarge, errno.CodeInvalidArg, "command is too large (max 8 KiB)")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), execTimeout)
	defer cancel()
	session, err := s.deps.Sessions.GetSession(ctx, sessionID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	if session.Kind != model.SessionKindSSH {
		writeError(w, http.StatusConflict, CodeCapabilityUnavailable, "exec is only available for SSH sessions")
		return
	}
	result, err := s.deps.Exec.RunBackgroundCommand(ctx, sessionID, command)
	if err != nil {
		writeAppError(w, err)
		return
	}

	output, truncated := truncateUTF8Text(result.Output, maxExecOutputBytes)
	writeJSON(w, http.StatusOK, ExecResponse{
		ExitCode:  result.ExitCode,
		Output:    output,
		Truncated: truncated,
	})
}

func (s *Server) handleSftpList(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if sessionID == "" || path == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "sessionId and path are required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	entries, err := s.deps.Files.ListRemoteDirectory(ctx, sessionID, path)
	if err != nil {
		writeAppError(w, err)
		return
	}
	truncated := false
	if len(entries) > maxListEntries {
		entries = entries[:maxListEntries]
		truncated = true
	}
	views := make([]SftpEntry, 0, len(entries))
	for _, entry := range entries {
		views = append(views, SftpEntry{
			Name:    entry.Name,
			Type:    string(entry.Type),
			Size:    entry.Size,
			ModTime: formatTime(entry.ModTime),
		})
	}
	writeJSON(w, http.StatusOK, SftpListResponse{Path: path, Entries: views, Truncated: truncated})
}

func (s *Server) handleSftpRead(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if sessionID == "" || path == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "sessionId and path are required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	content, err := s.deps.Files.ReadRemoteText(ctx, sessionID, path)
	if err != nil {
		writeAppError(w, err)
		return
	}

	if content.Binary {
		writeError(w, http.StatusConflict, CodeBinaryFile, "target is a binary file")
		return
	}
	data, decodeErr := base64.StdEncoding.DecodeString(content.ContentBase64)
	if decodeErr != nil {
		writeError(w, http.StatusInternalServerError, errno.CodeUnknown, "file content is not decodable")
		return
	}
	text, truncated := truncateUTF8Text(string(data), maxReadFileBytes)
	writeJSON(w, http.StatusOK, SftpReadResponse{
		Path:      path,
		Content:   text,
		Size:      content.Size,
		Truncated: truncated || content.Truncated,
		Hash:      content.Hash,
	})
}

func (s *Server) handleSftpWrite(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID    string `json:"sessionId"`
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expectedHash"`
	}
	if !decodeJSONBody(w, r, &payload) {
		return
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	path := strings.TrimSpace(payload.Path)
	if sessionID == "" || path == "" {
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "sessionId and path are required")
		return
	}
	if len(payload.Content) > maxWriteFileBytes {
		writeError(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge, "content is too large (max 256 KiB)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	result, err := s.deps.Files.WriteRemoteText(
		ctx,
		sessionID,
		path,
		base64.StdEncoding.EncodeToString([]byte(payload.Content)),
		strings.TrimSpace(payload.ExpectedHash),
	)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, SftpWriteResponse{
		Saved:       result.Saved,
		Conflict:    result.Conflict,
		CurrentHash: result.CurrentHash,
	})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge, "request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, errno.CodeInvalidArg, "invalid json body")
		return false
	}
	return true
}

func (s *Server) hostNames() map[string]string {
	hosts, err := s.deps.Hosts.ListHosts()
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(hosts))
	for _, host := range hosts {
		names[host.ID] = host.Name
	}
	return names
}

func projectSession(session model.Session, titles map[string]string) SessionView {
	return SessionView{
		ID:     session.ID,
		HostID: session.HostID,
		Title:  titles[session.HostID],
		Kind:   string(session.Kind),
		State:  string(session.State),
	}
}

func truncateUTF8Text(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
