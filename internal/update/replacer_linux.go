//go:build linux

package update

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func (s *Service) launchReplacer(_ context.Context, stagedPath, executable string) error {
	script, err := writeHelperScript(s.helperScriptDir(), "qterm-update.sh", linuxReplacerScript())
	if err != nil {
		slog.Warn("update replacer script write failed",
			"event", "update.replace.script_failed",
			"target", executable,
			"error", err,
		)
		return installFailed("helper")
	}
	command := exec.Command("/bin/sh", script, strconv.Itoa(os.Getpid()), executable, stagedPath)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		slog.Warn("update replacer spawn failed",
			"event", "update.replace.spawn_failed",
			"target", executable,
			"error", err,
		)
		return installFailed("spawn")
	}
	slog.Info("update replacer spawned",
		"event", "update.replace.spawned",
		"target", executable,
		"method", "shell_script",
	)
	return command.Process.Release()
}
