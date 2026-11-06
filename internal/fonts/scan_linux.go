//go:build linux

package fonts

import (
	"os"
	"path/filepath"
)

func fontDirectories() []string {
	directories := []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		directories = append(directories,
			filepath.Join(home, ".local", "share", "fonts"),
			filepath.Join(home, ".fonts"),
		)
	}
	return directories
}
