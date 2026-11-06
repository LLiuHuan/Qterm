//go:build !windows

package tunnel

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

var loginShells = map[string]struct{}{
	"sh":   {},
	"bash": {},
	"zsh":  {},
	"ksh":  {},
	"mksh": {},
	"dash": {},
	"fish": {},
}

func defaultLocalShellCandidates() []string {
	candidates := []string{os.Getenv("SHELL")}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/bin/zsh", "/bin/bash")
	} else {
		candidates = append(candidates, "/bin/bash", "/bin/zsh")
	}
	return append(candidates, "/bin/sh")
}

func localShellArgs(path string) []string {
	if _, ok := loginShells[filepath.Base(path)]; ok {
		return []string{"-l"}
	}
	return nil
}

func isLocalPTYReadClosed(err error) bool {
	return errors.Is(err, syscall.EIO)
}
