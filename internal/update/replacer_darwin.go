//go:build darwin

package update

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func (s *Service) launchReplacer(_ context.Context, stagedPath, executable string) error {
	bundle, ok := darwinBundleFromExecutable(executable)
	if !ok {

		return unsupportedPlatform("not-app-bundle")
	}
	script, err := writeHelperScript(s.helperScriptDir(), "qterm-update.sh", darwinReplacerScript())
	if err != nil {
		return installFailed("helper")
	}
	command := exec.Command("/bin/sh", script, strconv.Itoa(os.Getpid()), bundle, stagedPath)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return installFailed("spawn")
	}

	return command.Process.Release()
}
