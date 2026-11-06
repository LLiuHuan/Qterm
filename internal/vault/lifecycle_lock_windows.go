//go:build windows

package vault

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockLifecycleFile(file *os.File) error {
	if file == nil {
		return fmt.Errorf("%w: lock file is nil", ErrLifecycleLockUnavailable)
	}
	var overlapped windows.Overlapped
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return ErrLifecycleLockHeld
		}
		return fmt.Errorf("%w: acquire lock: %v", ErrLifecycleLockUnavailable, err)
	}
	return nil
}

func unlockLifecycleFile(file *os.File) error {
	if file == nil {
		return nil
	}
	var overlapped windows.Overlapped
	if err := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped); err != nil {
		return fmt.Errorf("release vault lifecycle lock: %w", err)
	}
	return nil
}
