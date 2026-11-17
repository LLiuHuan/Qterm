package app

import (
	"log/slog"
	"os"

	"changeme/internal/update"
)

func recoverFromFailedUpdate() {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		return
	}
	if update.TryRollbackAfterFailedUpdate(executable, relaunchExecutable) {
		slog.Warn("update rolled back to previous version", "executable", executable)
		os.Exit(0)
	}
}

func confirmUpdateStartup() {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		return
	}
	update.ConfirmStartup(executable)
}
