//go:build windows

package fonts

import (
	"os"
	"path/filepath"
)

func fontDirectories() []string {
	directories := make([]string, 0, 2)
	if windowsDir := os.Getenv("WINDIR"); windowsDir != "" {
		directories = append(directories, filepath.Join(windowsDir, "Fonts"))
	} else {
		directories = append(directories, `C:\Windows\Fonts`)
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		directories = append(directories, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
	}
	return directories
}
