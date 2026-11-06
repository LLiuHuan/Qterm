package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/recording"
	storage "changeme/internal/store"
)

const (
	terminalRecordingExportTXT       = "txt"
	terminalRecordingExportAsciicast = "asciicast"
	terminalRecordingExportMaxTries  = 10000
)

func (s *HostService) ExportTerminalRecordings(ids []string, format, targetDir string) error {

	return s.ExportTerminalRecordingsWithOptions(ids, format, targetDir, true)
}

func (s *HostService) ExportTerminalRecordingsWithOptions(ids []string, format, targetDir string, includeOutput bool) error {
	format, err := normalizeTerminalRecordingExportFormat(format)
	if err != nil {
		return fmt.Errorf("normalize terminal recording export format: %w", err)
	}
	idList := normalizeTerminalRecordingIDs(ids)
	if len(idList) == 0 {
		return errno.New(errno.CodeInvalidArg, "host.recording_ids_required", "")
	}
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" || strings.ContainsRune(targetDir, 0) {
		return errno.New(errno.CodeInvalidArg, "host.recording_export_path_invalid", "")
	}
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return errno.New(errno.CodeInvalidArg, "host.recording_export_path_invalid", "")
	}

	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if rootErr != nil {
		return rootErr
	}

	items := make([]model.TerminalRecording, 0, len(idList))
	for _, id := range idList {
		item, lookupErr := store.GetTerminalRecording(context.Background(), id)
		if lookupErr != nil {
			if errors.Is(lookupErr, storage.ErrNotFound) {
				return errno.New(errno.CodeNotFound, "host.recording_not_found", "")
			}
			return errno.Wrap(errno.CodeStoreFailed, "host.recording_lookup_failed", lookupErr)
		}
		if item.Status == "recording" || item.Status == "paused" {
			return errno.New(errno.CodeInvalidArg, "host.recording_active", "")
		}
		if _, pathErr := resolveRecordingPath(recordingsRoot, item.Path); pathErr != nil {
			return pathErr
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})

	writtenPaths := make([]string, 0, len(items))
	for _, item := range items {
		path, writeErr := exportTerminalRecording(targetDir, item, format, includeOutput, recordingsRoot)
		if writeErr != nil {
			for _, writtenPath := range writtenPaths {
				_ = os.Remove(writtenPath)
			}
			return writeErr
		}
		writtenPaths = append(writtenPaths, path)
	}
	return nil
}

func normalizeTerminalRecordingExportFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case terminalRecordingExportTXT:
		return terminalRecordingExportTXT, nil
	case terminalRecordingExportAsciicast, "asciicast-v2", "asciinema", "cast":
		return terminalRecordingExportAsciicast, nil
	default:
		return "", errno.New(errno.CodeInvalidArg, "host.recording_export_format_invalid", "")
	}
}

func normalizeTerminalRecordingIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func exportTerminalRecording(targetDir string, item model.TerminalRecording, format string, includeOutput bool, recordingsRoot string) (string, error) {
	sourcePath, err := resolveRecordingPath(recordingsRoot, item.Path)
	if err != nil {
		return "", fmt.Errorf("resolve terminal recording path: %w", err)
	}
	base := terminalRecordingExportBaseName(item)
	extension := ".txt"
	if format == terminalRecordingExportAsciicast {
		extension = ".cast"
	}
	for attempt := 0; attempt < terminalRecordingExportMaxTries; attempt++ {
		name := base + extension
		if attempt > 0 {
			name = fmt.Sprintf("%s-%d%s", base, attempt+1, extension)
		}
		targetPath := filepath.Join(targetDir, name)
		file, openErr := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(openErr, os.ErrExist) {
			continue
		}
		if openErr != nil {
			return "", errno.Wrap(errno.CodePermissionDenied, "host.recording_export_failed", openErr)
		}
		committed := false
		defer func() {
			if !committed {
				_ = file.Close()
				_ = os.Remove(targetPath)
			}
		}()

		if writeErr := writeTerminalRecording(file, sourcePath, format, includeOutput); writeErr != nil {
			return "", writeErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", closeErr)
		}
		committed = true
		return targetPath, nil
	}
	return "", errno.New(errno.CodeStoreFailed, "host.recording_export_failed", "")
}

func terminalRecordingExportBaseName(item model.TerminalRecording) string {
	host := sanitizeTerminalRecordingFilenamePart(item.HostName)
	if host == "" {
		host = sanitizeTerminalRecordingFilenamePart(item.HostID)
	}
	if host == "" {
		host = "recording"
	}
	startedAt := item.StartedAt.UTC().Format("20060102-150405")
	return host + "-" + startedAt
}

func sanitizeTerminalRecordingFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	return strings.Trim(builder.String(), ".")
}

func writeTerminalRecording(target *os.File, sourcePath, format string, includeOutput bool) error {
	if format == terminalRecordingExportAsciicast {
		return writeAsciicastV2(target, sourcePath, includeOutput)
	}
	reader, err := recording.Open(sourcePath)
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", err)
	}
	writtenErr := func() error {
		var copyErr error
		if includeOutput {
			_, copyErr = copyRecordingPlainOutput(target, reader)
		} else {
			_, copyErr = copyRecordingPlainInput(target, reader)
		}
		if copyErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", copyErr)
		}
		return nil
	}()
	if closeErr := reader.Close(); writtenErr == nil && closeErr != nil {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", closeErr)
	}
	return writtenErr
}

func copyRecordingPlainOutput(target io.Writer, reader *recording.Reader) (int64, error) {
	var written int64
	stripper := newANSITextStripper()
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return written, nil
		}
		if err != nil {
			return written, err
		}
		if event.Type == recording.EventEnd {
			return written, nil
		}
		if event.Type != recording.EventOutput || len(event.Data) == 0 {
			continue
		}
		clean := stripper.Write(event.Data)
		count, err := target.Write(clean)
		written += int64(count)
		if err != nil {
			return written, err
		}
	}
}

func copyRecordingPlainInput(target io.Writer, reader *recording.Reader) (int64, error) {
	var written int64
	stripper := newANSITextStripper()
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return written, nil
		}
		if err != nil {
			return written, err
		}
		if event.Type == recording.EventEnd {
			return written, nil
		}
		if event.Type != recording.EventInput || len(event.Data) == 0 {
			continue
		}
		clean := stripper.Write(event.Data)
		count, err := target.Write(clean)
		written += int64(count)
		if err != nil {
			return written, err
		}
	}
}

type ansiTextStripper struct {
	state byte
}

func newANSITextStripper() *ansiTextStripper { return &ansiTextStripper{} }

func (s *ansiTextStripper) Write(data []byte) []byte {
	clean := make([]byte, 0, len(data))
	for index := 0; index < len(data); index++ {
		char := data[index]
		switch s.state {
		case 0:
			if char == 0x1b {
				s.state = 1
				continue
			}
			if char == 0x9b {
				s.state = 2
				continue
			}
			clean = append(clean, char)
		case 1:
			switch char {
			case '[':
				s.state = 2
			case ']':
				s.state = 3
			case '(', ')', '*', '+', '-', '.', '/':
				s.state = 4
			default:
				s.state = 0
			}
		case 2:
			if char >= 0x40 && char <= 0x7e {
				s.state = 0
			}
		case 3:
			if char == 0x07 {
				s.state = 0
			} else if char == 0x1b {
				s.state = 5
			}
		case 4:
			s.state = 0
		case 5:
			if char == '\\' {
				s.state = 0
			} else {
				s.state = 3
			}
		}
	}
	return clean
}

func copyRecordingInput(target io.Writer, reader *recording.Reader) (int64, error) {
	var written int64
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return written, nil
		}
		if err != nil {
			return written, err
		}
		if event.Type == recording.EventEnd {
			return written, nil
		}
		if event.Type != recording.EventInput || len(event.Data) == 0 {
			continue
		}
		count, err := target.Write(event.Data)
		written += int64(count)
		if err != nil {
			return written, err
		}
	}
}

type asciicastRecordingInfo struct {
	startedAt time.Time
	width     uint16
	height    uint16
	duration  time.Duration
}

func writeAsciicastV2(target io.Writer, sourcePath string, includeOutput bool) error {
	info, err := inspectAsciicastRecording(sourcePath)
	if err != nil {
		return fmt.Errorf("inspect asciicast recording: %w", err)
	}
	width, height := info.width, info.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	header := struct {
		Version   int               `json:"version"`
		Width     uint16            `json:"width"`
		Height    uint16            `json:"height"`
		Timestamp int64             `json:"timestamp"`
		Duration  float64           `json:"duration"`
		Env       map[string]string `json:"env,omitempty"`
	}{
		Version:   2,
		Width:     width,
		Height:    height,
		Timestamp: info.startedAt.Unix(),
		Duration:  info.duration.Seconds(),
		Env:       map[string]string{"TERM": "xterm-256color"},
	}
	encoder := json.NewEncoder(target)
	if err := encoder.Encode(header); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
	}

	reader, err := recording.Open(sourcePath)
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", err)
	}
	writeErr := func() error {
		for {
			event, nextErr := reader.Next()
			if errors.Is(nextErr, io.EOF) {
				return nil
			}
			if nextErr != nil {
				return errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", nextErr)
			}
			if event.Type == recording.EventEnd {
				return nil
			}
			var eventType string
			var data string
			switch event.Type {
			case recording.EventOutput:
				if !includeOutput {
					continue
				}
				eventType = "o"
				data = string(event.Data)
			case recording.EventInput:
				eventType = "i"
				data = string(event.Data)
			case recording.EventResize:
				if !includeOutput {
					continue
				}
				eventType = "r"
				data = fmt.Sprintf("%dx%d", event.Columns, event.Rows)
			default:
				continue
			}
			if err := encoder.Encode([]any{event.At.Seconds(), eventType, data}); err != nil {
				return errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
			}
		}
	}()
	if closeErr := reader.Close(); writeErr == nil && closeErr != nil {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", closeErr)
	}
	return writeErr
}

func inspectAsciicastRecording(sourcePath string) (asciicastRecordingInfo, error) {
	reader, err := recording.Open(sourcePath)
	if err != nil {
		return asciicastRecordingInfo{}, errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", err)
	}
	info := asciicastRecordingInfo{startedAt: reader.StartedAt()}
	inspectErr := func() error {
		for {
			event, nextErr := reader.Next()
			if errors.Is(nextErr, io.EOF) {
				return nil
			}
			if nextErr != nil {
				return errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", nextErr)
			}
			if event.At > info.duration {
				info.duration = event.At
			}
			if event.Type == recording.EventResize {
				if info.width == 0 && event.Columns != 0 {
					info.width = event.Columns
				}
				if info.height == 0 && event.Rows != 0 {
					info.height = event.Rows
				}
			}
			if event.Type == recording.EventEnd {
				return nil
			}
		}
	}()
	if closeErr := reader.Close(); inspectErr == nil && closeErr != nil {
		return asciicastRecordingInfo{}, errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", closeErr)
	}
	if inspectErr != nil {
		return asciicastRecordingInfo{}, inspectErr
	}
	return info, nil
}
