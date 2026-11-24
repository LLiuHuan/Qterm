//go:build windows

package remotedesktop

import (
	"os/exec"
	"syscall"
)

func hideHelperProcessWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
