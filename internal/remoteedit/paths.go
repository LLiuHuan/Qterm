package remoteedit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	cacheRootRelative = "qterm/remote-edit"

	editDirectoryNameBytes = 16

	maxSanitizedStemBytes = 96

	maxSanitizedExtensionBytes = 16

	fallbackBaseName = "untitled"
)

const windowsIllegalNameChars = `<>:"/\|?*`

var windowsReservedNames = func() map[string]bool {
	names := make(map[string]bool, 22)
	for _, name := range []string{"con", "prn", "aux", "nul"} {
		names[name] = true
	}
	for _, prefix := range []string{"com", "lpt"} {
		for digit := 1; digit <= 9; digit++ {
			names[prefix+string(rune('0'+digit))] = true
		}
	}
	return names
}()

func RemoteEditRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(cache) {
		return "", errors.New("user cache directory is not absolute")
	}
	return filepath.Join(cache, filepath.FromSlash(cacheRootRelative)), nil
}

func editDirectoryName(sessionID string, remotePath string) string {
	sum := sha256.Sum256([]byte(sessionID + "\x00" + remotePath))
	return hex.EncodeToString(sum[:])[:editDirectoryNameBytes]
}

func (m *Manager) localPathFor(sessionID string, remotePath string) string {
	return filepath.Join(m.cacheRoot(), editDirectoryName(sessionID, remotePath), sanitizeBaseName(path.Base(remotePath)))
}

func sanitizeBaseName(name string) string {
	if name == "" || name == "." || name == ".." {
		name = fallbackBaseName
	}
	var builder strings.Builder
	builder.Grow(len(name))
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f || strings.ContainsRune(windowsIllegalNameChars, r):
			builder.WriteRune('_')
		default:
			builder.WriteRune(r)
		}
	}
	cleaned := strings.TrimRight(builder.String(), ". ")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		cleaned = fallbackBaseName
	}
	extension := filepath.Ext(cleaned)
	stem := strings.TrimSuffix(cleaned, extension)
	if len(extension) > maxSanitizedExtensionBytes {

		extension = ""
		stem = cleaned
	}
	if len(stem) > maxSanitizedStemBytes {
		stem = truncateBytes(stem, maxSanitizedStemBytes)
	}
	if stem == "" {
		stem = "file"
	}
	cleaned = stem + extension
	if windowsReservedNames[strings.ToLower(stem)] {

		cleaned = "_" + cleaned
	}
	return cleaned
}

func truncateBytes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	truncated := value[:limit]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func ensurePrivateDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}

	return os.Chmod(directory, 0o700)
}

func cleanOrphanDirectories(root string, keep map[string]bool) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	cleaned := filepath.Clean(root)
	for _, entry := range entries {
		if keep[entry.Name()] {
			continue
		}
		target := filepath.Join(cleaned, entry.Name())

		if filepath.Dir(target) != cleaned {
			continue
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}

func removeLocalEditFile(localPath string) error {
	var firstErr error
	if err := os.Remove(localPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		firstErr = err
	}

	if err := os.Remove(filepath.Dir(localPath)); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
