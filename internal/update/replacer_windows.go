//go:build windows

package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

func (s *Service) launchReplacer(_ context.Context, stagedPath, executable string) error {

	if err := verifyTargetWritable(executable); err != nil {
		return err
	}
	if err := applyStagedExecutable(stagedPath, executable); err != nil {
		return err
	}
	relaunchExecutable(executable)

	if dir := filepath.Dir(stagedPath); dir != "" {
		_ = os.RemoveAll(dir)
	}
	return nil
}

func relaunchExecutable(executable string) {
	command := exec.Command("explorer.exe", executable)
	if err := command.Start(); err != nil {
		return
	}
	_ = command.Process.Release()
}

func verifyTargetWritable(executable string) error {
	probe, err := os.CreateTemp(filepath.Dir(executable), ".qterm-write-probe-*")
	if err != nil {
		return installFailed("target-readonly")
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func cleanupStaleUpdateFiles(executable string) {
	_ = os.Remove(executable + ".old")
	_ = os.Remove(executable + ".new")
}
