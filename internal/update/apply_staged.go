package update

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func applyStagedExecutable(archivePath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return installFailed("archive")
	}
	defer func() { _ = reader.Close() }()

	entry := findExecutableEntry(reader.File, targetPath)
	if entry == nil {
		return installFailed("payload")
	}

	staged := targetPath + ".new"

	_ = os.Remove(staged)
	size, err := extractZipEntry(entry, staged)
	if err != nil {
		_ = os.Remove(staged)
		return installFailed("extract")
	}

	if size != int64(entry.UncompressedSize64) {
		_ = os.Remove(staged)
		return installFailed("extract")
	}

	backup := targetPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(targetPath, backup); err != nil {
		_ = os.Remove(staged)
		return installFailed("replace-backup")
	}
	if err := os.Rename(staged, targetPath); err != nil {

		_ = os.Rename(backup, targetPath)
		_ = os.Remove(staged)
		return installFailed("replace-swap")
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
