package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	ErrLifecycleLockHeld        = errors.New("vault lifecycle lock is already held")
	ErrLifecycleLockUnavailable = errors.New("vault lifecycle lock is unavailable")
)

type LifecycleLock struct {
	mu   sync.Mutex
	file *os.File
}

func AcquireLifecycleLock(path string) (*LifecycleLock, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("%w: lock path must be absolute", ErrLifecycleLockUnavailable)
	}
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	volumeRoot := filepath.Clean(filepath.VolumeName(dir) + string(os.PathSeparator))
	if path == dir || dir == volumeRoot {
		return nil, fmt.Errorf("%w: lock path cannot be in a filesystem root", ErrLifecycleLockUnavailable)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("%w: create lock directory: %v", ErrLifecycleLockUnavailable, err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return nil, fmt.Errorf("%w: protect lock directory: %v", ErrLifecycleLockUnavailable, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("%w: protect lock directory: %v", ErrLifecycleLockUnavailable, err)
		}
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: lock path is not a regular file", ErrLifecycleLockUnavailable)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: inspect lock file: %v", ErrLifecycleLockUnavailable, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: open lock file: %v", ErrLifecycleLockUnavailable, err)
	}
	openedInfo, statErr := file.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if statErr != nil || pathErr != nil || !openedInfo.Mode().IsRegular() || !pathInfo.Mode().IsRegular() || !os.SameFile(openedInfo, pathInfo) {
		_ = file.Close()
		if statErr != nil {
			return nil, fmt.Errorf("%w: inspect opened lock file: %v", ErrLifecycleLockUnavailable, statErr)
		}
		if pathErr != nil && !errors.Is(pathErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: inspect lock path after open: %v", ErrLifecycleLockUnavailable, pathErr)
		}
		return nil, fmt.Errorf("%w: lock path changed or is not a regular file", ErrLifecycleLockUnavailable)
	}
	if err := protectLocalPath(path, false); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%w: protect lock file: %v", ErrLifecycleLockUnavailable, err)
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("%w: protect lock file: %v", ErrLifecycleLockUnavailable, err)
		}
	}
	if err := lockLifecycleFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &LifecycleLock{file: file}, nil
}

func (l *LifecycleLock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	var result error
	if err := unlockLifecycleFile(file); err != nil {
		result = errors.Join(result, err)
	}
	if err := file.Close(); err != nil {
		result = errors.Join(result, fmt.Errorf("close vault lifecycle lock: %w", err))
	}
	return result
}
