//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

func relaunchExecutable(path string) error {
	command := exec.Command(path)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
