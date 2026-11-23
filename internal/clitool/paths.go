package clitool

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed skillfiles/SKILL.md
var skillDocument []byte

const envCLIPath = "QTERM_CLI_PATH"

func (s *Service) executablePath() string {
	if s != nil && s.execPath != "" {
		return s.execPath
	}
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		return resolved
	}
	return executable
}

func (s *Service) binaryCandidates() []string {
	var candidates []string
	if override := strings.TrimSpace(os.Getenv(envCLIPath)); override != "" {
		candidates = append(candidates, override)
	}
	executable := s.executablePath()
	if executable == "" {
		return candidates
	}
	directory := filepath.Dir(executable)
	name := commandFileName()
	candidates = append(candidates,

		filepath.Join(directory, name),

		filepath.Join(directory, "..", "Resources", "bin", name),

		filepath.Join(directory, "..", "lib", "qterm", name),

		filepath.Join(directory, name),
	)
	return candidates
}

func (s *Service) binaryOnPath(binary string) bool {
	if strings.TrimSpace(binary) == "" || !fileExists(binary) {
		return false
	}
	return pathContains(s.pathValue(), filepath.Dir(binary))
}

func (s *Service) binaryPath() (string, bool) {
	candidates := s.binaryCandidates()
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, true
		}
	}
	if len(candidates) > 0 {
		return candidates[0], false
	}
	return "", false
}

func (s *Service) installDir() string {
	if s != nil && s.installDirOverride != "" {
		return s.installDirOverride
	}
	pathValue := s.pathValue()
	if runtime.GOOS == "windows" {
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			base = s.homeDirectory()
		}
		return filepath.Join(base, "Qterm", "bin")
	}
	systemDir := "/usr/local/bin"
	userDir := filepath.Join(s.homeDirectory(), ".local", "bin")
	switch {
	case pathContains(pathValue, systemDir) && dirWritable(systemDir):
		return systemDir
	case pathContains(pathValue, userDir):
		return userDir
	case dirWritable(systemDir):
		return systemDir
	default:
		return userDir
	}
}

func (s *Service) skillsPath() string {
	return filepath.Join(s.homeDirectory(), ".agents", "skills", skillFolder, skillFileName)
}

func (s *Service) dirOnPath(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	return pathContains(s.pathValue(), dir)
}

func (s *Service) pathValue() string {
	if s != nil && s.pathEnv != "" {
		return s.pathEnv
	}
	return searchPathValue(os.Getenv("PATH"))
}

func processPath() string { return os.Getenv("PATH") }

func (s *Service) homeDirectory() string {
	if s != nil && s.homeDir != "" {
		return s.homeDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func pathContains(pathValue, dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	want := filepath.Clean(dir)
	wantResolved, resolveErr := filepath.EvalSymlinks(want)
	for _, entry := range filepath.SplitList(pathValue) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if samePath(entry, want) {
			return true
		}
		if resolveErr != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(filepath.Clean(entry)); err == nil && resolved == wantResolved {
			return true
		}
	}
	return false
}

func dirWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	probe, err := os.CreateTemp(dir, ".qterm-write-probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}
