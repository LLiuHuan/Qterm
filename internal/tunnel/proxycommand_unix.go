//go:build !windows

package tunnel

import (
	"os/exec"
	"syscall"
)

func shellFor(command string) *exec.Cmd {
	process := exec.Command("/bin/sh", "-c", command)

	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return process
}

func killProxyCommand(process *exec.Cmd) {
	if process == nil || process.Process == nil {
		return
	}
	if process.Process.Pid > 0 {
		_ = syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
	}
	_ = process.Process.Kill()
}
