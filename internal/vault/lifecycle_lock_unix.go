//go:build !windows

package vault

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockLifecycleFile(file *os.File) error {
	if file == nil {
		return fmt.Errorf("%w: lock file is nil", ErrLifecycleLockUnavailable)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
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
	if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("release vault lifecycle lock: %w", err)
	}
	return nil
}
