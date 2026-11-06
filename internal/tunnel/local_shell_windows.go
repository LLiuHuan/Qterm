//go:build windows

package tunnel

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultLocalShellCandidates() []string {
	candidates := []string{os.Getenv("SHELL")}
	if command := strings.TrimSpace(os.Getenv("COMSPEC")); command != "" {
		candidates = append(candidates, command)
	}
	return append(candidates, "pwsh.exe", "powershell.exe", "cmd.exe")
}

func localShellArgs(path string) []string {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, "pwsh") || strings.HasPrefix(name, "powershell") {
		return []string{"-NoLogo"}
	}
	return nil
}

func isLocalPTYReadClosed(error) bool {
	return false
}
