//go:build windows

package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func (s *Service) launchReplacer(ctx context.Context, stagedPath, executable string) error {
	writableErr := verifyTargetWritable(executable)
	if writableErr == nil {
		slog.Info("update target writable, replacing in place",
			"event", "update.replace.in_place",
			"target", executable,
		)
		if err := applyStagedExecutable(stagedPath, executable); err != nil {
			return err
		}

		_ = RecordPendingUpdate(executable)
		relaunchExecutable(executable)

		if dir := filepath.Dir(stagedPath); dir != "" {
			_ = os.RemoveAll(dir)
		}
		return nil
	} else {

		slog.Info("update target not writable, need elevation",
			"event", "update.replace.needs_elevation",
			"target", executable,
			"error", writableErr,
		)
	}

	return launchElevatedHelper(stagedPath, executable)
}

func relaunchExecutable(executable string) {
	command := exec.Command("explorer.exe", executable)
	if err := command.Start(); err != nil {
		return
	}
	_ = command.Process.Release()
}

const errnoCancelled syscall.Errno = 1223

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

	if _, _, pending := readPendingUpdate(executable); !pending {
		_ = os.Remove(executable + ".old")
	}
	_ = os.Remove(executable + ".new")
}

func launchElevatedHelper(stagedPath, executable string) error {
	dir, err := helperWorkDir()
	if err != nil {
		slog.Warn("update helper work dir unavailable",
			"event", "update.elevate.workdir_failed",
			"error", err,
		)
		return installFailed("elevation-staging")
	}
	archive := filepath.Join(dir, "qterm-update.zip")
	if err := copyFile(stagedPath, archive); err != nil {
		slog.Warn("update helper archive staging failed",
			"event", "update.elevate.staging_failed",
			"archive", archive,
			"error", err,
		)
		return installFailed("elevation-staging")
	}
	planPath := filepath.Join(dir, "update-plan.json")
	plan, err := writeElevatedPlan(planPath, archive, executable)
	if err != nil {
		_ = os.Remove(archive)
		slog.Warn("update helper plan write failed",
			"event", "update.elevate.plan_failed",
			"plan", planPath,
			"error", err,
		)
		return installFailed("elevation-staging")
	}

	self, err := os.Executable()
	if err != nil {
		_ = os.Remove(archive)
		RemoveUpdatePlan(planPath)
		slog.Warn("update helper executable path unavailable",
			"event", "update.elevate.executable_failed",
			"error", err,
		)
		return installFailed("elevation-spawn")
	}

	slog.Info("update helper launch requested",
		"event", "update.elevate.start",
		"plan", planPath,
		"archive", archive,
		"target", executable,
	)
	parameters := UpdateHelperFlagPrefix + quoteArg(planPath) + " " + planNoncePrefix + plan.Nonce
	if err := runElevated(self, parameters); err != nil {

		_ = os.Remove(archive)
		RemoveUpdatePlan(planPath)

		if code, ok := err.(syscall.Errno); ok && code == errnoCancelled {
			slog.Warn("update elevation cancelled by user",
				"event", "update.elevate.cancelled",
				"target", executable,
				"error_key", keyInstallFailed,
			)
			return installFailed("elevation-cancelled")
		}
		slog.Error("update elevation spawn failed",
			"event", "update.elevate.spawn_failed",
			"target", executable,
			"error", err,
		)
		return installFailed("elevation-spawn")
	}
	slog.Info("update helper launched",
		"event", "update.elevate.launched",
		"plan", planPath,
		"target", executable,
	)

	if dir := filepath.Dir(stagedPath); dir != "" {
		_ = os.RemoveAll(dir)
	}
	return nil
}

func helperWorkDir() (string, error) {
	root := os.Getenv("ProgramData")
	if root == "" {
		root = `C:\ProgramData`
	}
	dir := filepath.Join(root, "Qterm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func writeElevatedPlan(planPath, archive, executable string) (UpdatePlan, error) {
	RemoveUpdatePlan(planPath)
	digest, err := fileSHA256(archive)
	if err != nil {
		return UpdatePlan{}, err
	}
	nonce, err := randomNonce()
	if err != nil {
		return UpdatePlan{}, err
	}
	plan := UpdatePlan{
		Archive:       archive,
		ArchiveSHA256: digest,
		Target:        executable,
		Pid:           os.Getpid(),
		Nonce:         nonce,
	}
	if err := WriteUpdatePlan(planPath, plan); err != nil {
		return UpdatePlan{}, err
	}
	return plan, nil
}

func randomNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func runElevated(executable, arg string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	parameters, err := windows.UTF16PtrFromString(arg)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, parameters, nil, windows.SW_HIDE)
}

func quoteArg(value string) string {
	if value == "" {
		return `""`
	}
	return `"` + value + `"`
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	partial := destination + ".part"
	output, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = os.Remove(partial)
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		_ = os.Remove(partial)
		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := os.Rename(partial, destination); err != nil {
		_ = os.Remove(partial)
		return err
	}
	return nil
}
