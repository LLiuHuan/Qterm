//go:build !windows

package clitool

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func ensureSearchPath(dir, home string) error {
	return writeShellSearchPath(dir, home, os.Getenv("SHELL"), runtime.GOOS)
}

func removeSearchPath(dir, home string) error {
	return removeShellSearchPath(dir, home, os.Getenv("SHELL"), runtime.GOOS)
}

func pathSearchesDir(dir, pathValue string) bool {
	return unixDirOnUserPath(dir, pathValue)
}

var staleEntryDirs = func(home string) []string {
	return unixInstallCandidates(home, runtime.GOOS, runtime.GOARCH)
}

func removeStaleEntries(installDir, home string) error {
	var firstErr error
	for _, dir := range staleEntryDirs(home) {
		if samePath(dir, installDir) {
			continue
		}
		link := filepath.Join(dir, commandFileName())
		owned, exists := entryOwned(link)
		if !exists || !owned {
			continue
		}
		if err := removeEntry(link); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func shellPathFile(dir, home string) string {
	return shellSearchPathFile(dir, home, os.Getenv("SHELL"), runtime.GOOS)
}

func searchPathValue(fallback string) string {
	return mergePathLists(fallback, systemSearchDirs())
}

func systemSearchDirs() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return readPathLists("/etc/paths", "/etc/paths.d")
}

func readPathLists(pathsFile, pathsDir string) []string {
	dirs := parsePathListFile(pathsFile)
	entries, err := os.ReadDir(pathsDir)
	if err != nil {
		return dirs
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dirs = append(dirs, parsePathListFile(filepath.Join(pathsDir, entry.Name()))...)
	}
	return dirs
}

func parsePathListFile(path string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dirs = append(dirs, line)
	}
	return dirs
}

func mergePathLists(base string, extra []string) string {
	parts := make([]string, 0, len(extra)+2)
	for _, entry := range filepath.SplitList(base) {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		parts = append(parts, entry)
	}
	for _, dir := range extra {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		present := false
		for _, entry := range parts {
			if samePath(entry, dir) {
				present = true
				break
			}
		}
		if !present {
			parts = append(parts, dir)
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
