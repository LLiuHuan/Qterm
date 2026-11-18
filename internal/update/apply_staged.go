package update

import (
	"archive/zip"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type StagedFailure struct {
	Detail string
}

func (f *StagedFailure) Error() string {
	return "staged update failed: " + f.Detail
}

func applyStagedExecutable(archivePath, targetPath string) error {
	if err := ApplyStagedArchive(archivePath, targetPath); err != nil {
		var failure *StagedFailure
		if errors.As(err, &failure) {
			return installFailed(failure.Detail)
		}
		return installFailed("archive")
	}
	return nil
}

func ApplyStagedArchive(archivePath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {

		slog.Error("update staged archive open failed",
			"event", "update.apply.archive_failed",
			"archive", archivePath,
			"error", err,
		)
		return &StagedFailure{Detail: "archive"}
	}
	defer func() { _ = reader.Close() }()

	entry := findExecutableEntry(reader.File, targetPath)
	if entry == nil {
		slog.Error("update staged archive has no executable",
			"event", "update.apply.payload_missing",
			"archive", archivePath,
			"target", targetPath,
		)
		return &StagedFailure{Detail: "payload"}
	}

	staged := targetPath + ".new"

	_ = os.Remove(staged)
	size, err := extractZipEntry(entry, staged)
	if err != nil {
		_ = os.Remove(staged)
		slog.Error("update staged archive extract failed",
			"event", "update.apply.extract_failed",
			"archive", archivePath,
			"target", targetPath,
			"error", err,
		)
		return &StagedFailure{Detail: "extract"}
	}

	if size != int64(entry.UncompressedSize64) {
		_ = os.Remove(staged)
		slog.Error("update staged payload size mismatch",
			"event", "update.apply.size_mismatch",
			"target", targetPath,
			"bytes", size,
			"expected", int64(entry.UncompressedSize64),
		)
		return &StagedFailure{Detail: "extract"}
	}

	backup := targetPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(targetPath, backup); err != nil {
		_ = os.Remove(staged)
		slog.Error("update replace backup rename failed",
			"event", "update.apply.backup_failed",
			"target", targetPath,
			"backup", backup,
			"error", err,
		)
		return &StagedFailure{Detail: "replace-backup"}
	}
	if err := os.Rename(staged, targetPath); err != nil {

		rollbackErr := os.Rename(backup, targetPath)
		_ = os.Remove(staged)
		slog.Error("update replace swap failed",
			"event", "update.apply.swap_failed",
			"target", targetPath,
			"backup", backup,
			"restored", rollbackErr == nil,
			"error", err,
		)
		return &StagedFailure{Detail: "replace-swap"}
	}

	_ = os.Remove(backup)
	return nil
}

func findExecutableEntry(entries []*zip.File, targetPath string) *zip.File {
	want := strings.ToLower(filepath.Base(targetPath))
	var fallback *zip.File
	for _, entry := range entries {
		if entry.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.Base(entry.Name))
		if name == want {
			return entry
		}
		if fallback == nil && strings.HasSuffix(name, ".exe") {
			fallback = entry
		}
	}
	return fallback
}

func extractZipEntry(entry *zip.File, destPath string) (int64, error) {
	source, err := entry.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = source.Close() }()

	destination, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(destination, source)

	if syncErr := destination.Sync(); copyErr == nil {
		copyErr = syncErr
	}
	if closeErr := destination.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return written, copyErr
}
