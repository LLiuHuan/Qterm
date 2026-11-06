//go:build windows

package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func (s *Service) launchReplacer(_ context.Context, stagedPath, executable string) error {
	script, err := writeHelperScript(filepath.Dir(stagedPath), "qterm-update.ps1", windowsReplacerScript())
	if err != nil {
		return installFailed("helper")
	}
	command := exec.Command(
		"cmd", "/c", "start", "", "/b",
		"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
		"-File", script,
		"-ProcessId", strconv.Itoa(os.Getpid()),
		"-Target", executable,
		"-Archive", stagedPath,
	)
	if err := command.Start(); err != nil {
		return installFailed("spawn")
	}
	return command.Process.Release()
}
