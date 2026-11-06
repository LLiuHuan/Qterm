//go:build darwin

package fonts

import (
	"os"
	"path/filepath"
)

func fontDirectories() []string {
	directories := []string{
		"/System/Library/Fonts",
		"/Library/Fonts",
	}

	for _, pattern := range []string{
		"/System/Library/AssetsV2/com_apple_MobileAsset_Font*",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		directories = append(directories, matches...)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		directories = append(directories, filepath.Join(home, "Library", "Fonts"))
	}
	return directories
}
