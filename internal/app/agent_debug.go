package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	agentDebugFileName = "ai-agent.log"
	agentDebugMaxBytes = 2 << 20
)

type agentDebugLog struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newAgentDebugLog(path string) *agentDebugLog {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		slog.Warn("create ai agent log directory failed", "error", err.Error())
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		slog.Warn("open ai agent log failed", "error", err.Error())
		return nil
	}
	size := int64(0)
	if info, statErr := file.Stat(); statErr == nil {
		size = info.Size()
	}
	return &agentDebugLog{path: path, file: file, size: size}
}

func (log *agentDebugLog) Log(runID, event, detail string) {
	if log == nil {
		return
	}
	line := fmt.Sprintf("%s [%s] %s: %s\n", time.Now().Format(time.RFC3339), runID, event, detail)
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.file == nil {
		return
	}
	written, err := log.file.WriteString(line)
	if err != nil {
		return
	}
	log.size += int64(written)
	if log.size >= agentDebugMaxBytes {
		log.rotateLocked()
	}
}

func (log *agentDebugLog) rotateLocked() {
	_ = log.file.Close()
	_ = os.Rename(log.path, log.path+".1")
	file, err := os.OpenFile(log.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		log.file = nil
		return
	}
	log.file = file
	log.size = 0
}

func (log *agentDebugLog) Close() {
	if log == nil {
		return
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.file != nil {
		_ = log.file.Close()
		log.file = nil
	}
}
