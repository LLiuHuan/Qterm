//go:build windows

package tunnel

import (
	"os"
	"os/exec"
	"syscall"
)

func shellFor(command string) *exec.Cmd {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}
	process := exec.Command(shell, "/c", command)
	process.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return process
}

func killProxyCommand(process *exec.Cmd) {
	if process == nil || process.Process == nil {
		return
	}
	_ = process.Process.Kill()
}
