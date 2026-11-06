//go:build linux

package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func (s *Service) launchReplacer(_ context.Context, stagedPath, executable string) error {
	script, err := writeHelperScript(filepath.Dir(stagedPath), "qterm-update.sh", linuxReplacerScript())
	if err != nil {
		return installFailed("helper")
	}
	command := exec.Command("/bin/sh", script, strconv.Itoa(os.Getpid()), executable, stagedPath)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return installFailed("spawn")
	}
	return command.Process.Release()
}
