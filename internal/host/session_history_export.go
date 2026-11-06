package host

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/recording"
	storage "changeme/internal/store"
)

func resolveRecordingPath(root string, path string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errno.New(errno.CodeStoreFailed, "host.recording_path_failed", "")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", errno.New(errno.CodeStoreFailed, "host.recording_path_failed", "")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errno.New(errno.CodePermissionDenied, "host.recording_path_invalid", "")
	}
	rel, err := filepath.Rel(rootAbs, absolute)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errno.New(errno.CodePermissionDenied, "host.recording_path_invalid", "")
	}
	return absolute, nil
}

func (s *HostService) ExportSessionHistoryText(sessionID string, targetPath string) (int64, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return 0, errno.New(errno.CodeInvalidArg, "host.recording_session_required", "")
	}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" || strings.ContainsRune(targetPath, 0) {
		return 0, errno.New(errno.CodeInvalidArg, "host.recording_export_path_invalid", "")
	}
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return 0, ErrHostServiceNotReady
	}
	if rootErr != nil {
		return 0, rootErr
	}

	history, historyErr := store.GetSessionHistory(context.Background(), sessionID)
	hasHistory := historyErr == nil
	if historyErr != nil && !errors.Is(historyErr, storage.ErrNotFound) {
		return 0, errno.Wrap(errno.CodeStoreFailed, "history.list_failed", historyErr)
	}

	items, err := store.ListTerminalRecordings(context.Background())
	if err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.list_recordings_failed", err)
	}
	selected := make([]model.TerminalRecording, 0, len(items))
	for _, item := range items {
		if item.SessionID != sessionID {
			continue
		}

		if item.Status == "recording" || item.Status == "paused" {
			return 0, errno.New(errno.CodeInvalidArg, "host.recording_active", "")
		}
		selected = append(selected, item)
	}
	if !hasHistory && len(selected) == 0 {
		return 0, errno.New(errno.CodeNotFound, "history.not_found", "")
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].StartedAt.Before(selected[j].StartedAt)
	})

	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, errno.Wrap(errno.CodePermissionDenied, "host.recording_export_failed", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(targetPath)
		}
	}()

	buffered := bufio.NewWriter(file)
	written := int64(0)
	writeText := func(text string) error {
		count, err := buffered.WriteString(text)
		written += int64(count)
		if err != nil {
			return errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
		}
		return nil
	}

	if err := writeText(buildSessionHistoryHeader(sessionID, history, hasHistory)); err != nil {
		return 0, err
	}
	for _, item := range selected {
		path, err := resolveRecordingPath(recordingsRoot, item.Path)
		if err != nil {
			return 0, err
		}
		header := fmt.Sprintf("## 终端录制 %s（开始 %s，%d 字节 / %d 事件）\n", item.ID, item.StartedAt.UTC().Format(time.RFC3339), item.Bytes, item.EventCount)
		if err := writeText(header); err != nil {
			return 0, err
		}
		reader, err := recording.Open(path)
		if err != nil {
			return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", err)
		}
		outputCount, copyErr := copyRecordingOutput(buffered, reader)
		written += outputCount
		_ = reader.Close()
		if copyErr != nil {
			return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", copyErr)
		}
		if err := writeText("\n"); err != nil {
			return 0, err
		}
	}
	if err := buffered.Flush(); err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
	}
	if err := file.Close(); err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
	}
	committed = true
	return written, nil
}

func buildSessionHistoryHeader(sessionID string, history model.SessionHistory, hasHistory bool) string {
	var builder strings.Builder
	builder.WriteString("# Qterm 会话记录\n")
	if !hasHistory {
		builder.WriteString(fmt.Sprintf("会话：%s（缺少历史元数据）\n\n", sessionID))
		return builder.String()
	}
	builder.WriteString(fmt.Sprintf("主机：%s\n", history.HostName))
	builder.WriteString(fmt.Sprintf("地址：%s\n", history.Address))
	builder.WriteString(fmt.Sprintf("状态：%s\n", history.Status))
	builder.WriteString(fmt.Sprintf("开始：%s\n", history.StartedAt.UTC().Format(time.RFC3339)))
	if !history.EndedAt.IsZero() {
		builder.WriteString(fmt.Sprintf("结束：%s\n", history.EndedAt.UTC().Format(time.RFC3339)))
	}
	if strings.TrimSpace(history.Note) != "" {
		builder.WriteString(fmt.Sprintf("备注：%s\n", history.Note))
	}
	if strings.TrimSpace(history.ErrorKey) != "" {
		builder.WriteString(fmt.Sprintf("错误：%s\n", history.ErrorKey))
	}
	builder.WriteString("\n")
	return builder.String()
}

func copyRecordingOutput(target io.Writer, reader *recording.Reader) (int64, error) {
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
		if event.Type != recording.EventOutput || len(event.Data) == 0 {
			continue
		}
		count, err := target.Write(event.Data)
		written += int64(count)
		if err != nil {
			return written, err
		}
	}
}

func (s *HostService) ExportTerminalRecordingsText(ids []string, targetPath string) (int64, error) {
	idList := normalizeTerminalRecordingIDs(ids)
	if len(idList) == 0 {
		return 0, errno.New(errno.CodeInvalidArg, "host.recording_ids_required", "")
	}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" || strings.ContainsRune(targetPath, 0) {
		return 0, errno.New(errno.CodeInvalidArg, "host.recording_export_path_invalid", "")
	}
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return 0, ErrHostServiceNotReady
	}
	if rootErr != nil {
		return 0, rootErr
	}

	ctx := context.Background()
	items, err := store.GetTerminalRecordings(ctx, idList)
	if err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.list_recordings_failed", err)
	}
	if len(items) == 0 {
		return 0, errno.New(errno.CodeNotFound, "host.recording_not_found", "")
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})

	histories := make(map[string]sessionHistoryLookup, len(items))
	for _, item := range items {
		if _, ok := histories[item.SessionID]; ok {
			continue
		}
		history, historyErr := store.GetSessionHistory(ctx, item.SessionID)
		if historyErr != nil && !errors.Is(historyErr, storage.ErrNotFound) {
			return 0, errno.Wrap(errno.CodeStoreFailed, "history.list_failed", historyErr)
		}
		histories[item.SessionID] = sessionHistoryLookup{history: history, found: historyErr == nil}
	}

	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, errno.Wrap(errno.CodePermissionDenied, "host.recording_export_failed", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(targetPath)
		}
	}()

	buffered := bufio.NewWriter(file)
	written := int64(0)
	writeText := func(text string) error {
		count, writeErr := buffered.WriteString(text)
		written += int64(count)
		if writeErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", writeErr)
		}
		return nil
	}

	if err := writeText(fmt.Sprintf("# Qterm 终端录制导出（%d 段）\n\n", len(items))); err != nil {
		return 0, err
	}
	for index, item := range items {
		lookup := histories[item.SessionID]
		if err := writeText(buildRecordingExportSection(index+1, item, lookup)); err != nil {
			return 0, err
		}
		path, pathErr := resolveRecordingPath(recordingsRoot, item.Path)
		if pathErr != nil {
			return 0, pathErr
		}
		reader, openErr := recording.Open(path)
		if openErr != nil {
			return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", openErr)
		}
		outputCount, copyErr := copyRecordingOutput(buffered, reader)
		written += outputCount
		_ = reader.Close()
		if copyErr != nil {
			return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", copyErr)
		}
		if err := writeText("\n"); err != nil {
			return 0, err
		}
	}
	if err := buffered.Flush(); err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
	}
	if err := file.Close(); err != nil {
		return 0, errno.Wrap(errno.CodeStoreFailed, "host.recording_export_failed", err)
	}
	committed = true
	return written, nil
}

type sessionHistoryLookup struct {
	history model.SessionHistory
	found   bool
}

func buildRecordingExportSection(index int, item model.TerminalRecording, lookup sessionHistoryLookup) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("## 录制 %d\n", index))
	if lookup.found {
		history := lookup.history
		builder.WriteString(fmt.Sprintf("主机：%s\n", history.HostName))
		builder.WriteString(fmt.Sprintf("地址：%s\n", history.Address))
		builder.WriteString(fmt.Sprintf("状态：%s\n", history.Status))
		if strings.TrimSpace(history.Note) != "" {
			builder.WriteString(fmt.Sprintf("备注：%s\n", history.Note))
		}
	}
	builder.WriteString(fmt.Sprintf("录制：%s（开始 %s，%d 字节 / %d 事件）\n", item.ID, item.StartedAt.UTC().Format(time.RFC3339), item.Bytes, item.EventCount))
	builder.WriteString(strings.Repeat("-", 40))
	builder.WriteString("\n")
	return builder.String()
}
