//go:build darwin

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
	bundle, ok := darwinBundleFromExecutable(executable)
	if !ok {

		slog.Warn("update target is not an app bundle",
			"event", "update.replace.not_app_bundle",
			"target", executable,
		)
		return unsupportedPlatform("not-app-bundle")
	}
	script, err := writeHelperScript(s.helperScriptDir(), "qterm-update.sh", darwinReplacerScript())
	if err != nil {
		slog.Warn("update replacer script write failed",
			"event", "update.replace.script_failed",
			"target", bundle,
			"error", err,
		)
		return installFailed("helper")
	}
	command := exec.Command("/bin/sh", script, strconv.Itoa(os.Getpid()), bundle, stagedPath)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		slog.Warn("update replacer spawn failed",
			"event", "update.replace.spawn_failed",
			"target", bundle,
			"error", err,
		)
		return installFailed("spawn")
	}
	slog.Info("update replacer spawned",
		"event", "update.replace.spawned",
		"target", bundle,
		"method", "shell_script",
	)

	return command.Process.Release()
}
