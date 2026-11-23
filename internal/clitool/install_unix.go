//go:build !windows

package clitool

import (
	"os"
	"path/filepath"
)

func installEntry(binary, linkPath string) error {
	return os.Symlink(binary, linkPath)
}

func removeEntry(linkPath string) error {
	return os.Remove(linkPath)
}

func entryInstalled(binary, linkPath string) bool {
	return linkPointsTo(linkPath, binary)
}

func entryOwned(linkPath string) (owned bool, exists bool) {
	return ownLink(linkPath)
}

func managesSearchPath() bool { return false }

func ensureSearchPath(string) error { return nil }

func removeSearchPath(string) error { return nil }

func searchPathValue(fallback string) string { return fallback }

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
