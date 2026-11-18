package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"changeme/internal/update"
)

const (
	helperLogFileName = "qterm-update-helper.log"

	helperLogMaxBytes = 1 << 20

	helperLogTimeLayout = "2006-01-02T15:04:05.000Z07:00"
)

const helperLogRedacted = "<redacted>"

var helperLogger *helperLog

type helperLog struct {
	mu   sync.Mutex
	file *os.File

	path string

	size int64
}

func openHelperLog(plan update.UpdatePlan) *helperLog {
	primary, hasPrimary := helperLogPath()
	if hasPrimary {
		if entry, err := openHelperLogFile(primary); err == nil {
			return entry
		}
	}
	fallback, hasFallback := helperLogFallbackPath(plan)
	if hasFallback {
		if entry, err := openHelperLogFile(fallback); err == nil {
			if hasPrimary {

				entry.log("WARN", "helper.log.open_failed", "path", primary, "fallback", fallback)
			}
			return entry
		}
	}

	return nil
}

func helperLogPath() (string, bool) {
	root, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(root) == "" {
		return "", false
	}
	return filepath.Join(root, "Qterm", "logs", helperLogFileName), true
}

func helperLogFallbackPath(plan update.UpdatePlan) (string, bool) {
	dir := strings.TrimSpace(filepath.Dir(strings.TrimSpace(plan.Archive)))
	if dir == "" || dir == "." {
		return "", false
	}
	return filepath.Join(dir, helperLogFileName), true
}

func openHelperLogFile(path string) (*helperLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	var size int64
	if info, statErr := file.Stat(); statErr == nil {
		size = info.Size()
	}
	return &helperLog{file: file, path: path, size: size}, nil
}

func (l *helperLog) log(level, event string, kv ...any) {
	if l == nil {
		return
	}
	line := formatHelperLogLine(time.Now(), level, event, kv)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}

	if l.size+int64(len(line)) > helperLogMaxBytes {
		l.rotateLocked()
		if l.file == nil {
			return
		}
	}
	written, err := l.file.WriteString(line)
	if err != nil {
		return
	}
	l.size += int64(written)

	_ = l.file.Sync()
}

func (l *helperLog) close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func (l *helperLog) rotateLocked() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	if l.path == "" {
		return
	}
	_ = os.Remove(l.path + ".1")
	if err := os.Rename(l.path, l.path+".1"); err != nil {

		entry, openErr := openHelperLogFile(l.path)
		if openErr != nil {
			return
		}
		l.file = entry.file
		l.size = entry.size
		return
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	l.file = file
	l.size = 0
}

func formatHelperLogLine(now time.Time, level, event string, kv []any) string {
	builder := strings.Builder{}
	builder.WriteString(now.Format(helperLogTimeLayout))
	builder.WriteByte(' ')
	builder.WriteString(strings.ToUpper(strings.TrimSpace(level)))
	builder.WriteByte(' ')
	builder.WriteString(event)
	for index := 0; index+1 < len(kv); index += 2 {
		key := strings.TrimSpace(fmt.Sprint(kv[index]))
		if key == "" {
			continue
		}
		builder.WriteByte(' ')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(formatHelperLogValue(kv[index+1]))
	}
	builder.WriteByte('\n')
	return builder.String()
}

func formatHelperLogValue(value any) string {
	text := ""
	if err, ok := value.(error); ok && err != nil {
		text = err.Error()
	} else {
		text = fmt.Sprint(value)
	}
	text = strings.ReplaceAll(text, "\r", `\r`)
	text = strings.ReplaceAll(text, "\n", `\n`)
	if text == "" {
		return `""`
	}
	if strings.ContainsAny(text, " \t\"=") {
		return strconv.Quote(text)
	}
	return text
}
