//go:build !windows

package clitool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	shellPathBlockBegin = "# >>> qterm-cli >>>"
	shellPathBlockEnd   = "# <<< qterm-cli <<<"
)

type shellProfile struct {
	Shell string

	Path string

	Body string
}

func shellDisplayPath(dir, home string) string {
	dir = strings.TrimSpace(dir)
	home = strings.TrimSpace(home)
	if dir == "" {
		return ""
	}
	if home != "" && strings.HasPrefix(dir, home+string(filepath.Separator)) {
		relative := strings.TrimPrefix(dir, home+string(filepath.Separator))
		return "$HOME/" + filepath.ToSlash(relative)
	}
	return dir
}

func shellPathBody(shell, dir, home string) string {
	shown := shellDisplayPath(dir, home)
	if shown == "" {
		return ""
	}
	if shell == "fish" {

		return "if not contains \"" + shown + "\" $PATH\n    set -gx PATH \"" + shown + "\" $PATH\nend"
	}
	return "export PATH=\"" + shown + ":$PATH\""
}

func bashShellProfile(home, dir string, goos string) shellProfile {
	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}
	if goos == "darwin" {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}
	chosen := ""
	for _, candidate := range candidates {
		if fileExists(candidate) {
			chosen = candidate
			break
		}
	}
	if chosen == "" {
		chosen = candidates[0]
	}
	return shellProfile{Shell: "bash", Path: chosen, Body: shellPathBody("bash", dir, home)}
}

func shellPathProfiles(home, shell, dir, goos string) []shellProfile {
	if strings.TrimSpace(home) == "" || strings.TrimSpace(dir) == "" {
		return nil
	}
	all := map[string]shellProfile{
		"zsh":  {Shell: "zsh", Path: filepath.Join(home, ".zshrc"), Body: shellPathBody("zsh", dir, home)},
		"bash": bashShellProfile(home, dir, goos),
		"fish": {Shell: "fish", Path: filepath.Join(home, ".config", "fish", "config.fish"), Body: shellPathBody("fish", dir, home)},
		"sh":   {Shell: "sh", Path: filepath.Join(home, ".profile"), Body: shellPathBody("sh", dir, home)},
	}
	order := make([]string, 0, len(all))
	if current := normalizeShellName(shell); current != "" {
		order = append(order, current)
	}
	for _, name := range []string{"zsh", "bash", "fish", "sh"} {
		duplicated := false
		for _, existing := range order {
			if existing == name {
				duplicated = true
				break
			}
		}
		if !duplicated {
			order = append(order, name)
		}
	}
	profiles := make([]shellProfile, 0, len(order))
	for _, name := range order {
		profiles = append(profiles, all[name])
	}
	return profiles
}

func normalizeShellName(shell string) string {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(shell)))
	switch name {
	case "zsh", "bash", "fish":
		return name
	case "sh", "dash", "ash", "ksh", "mksh":
		return "sh"
	default:
		return ""
	}
}

func activeShellProfile(home, shell, dir, goos string) (shellProfile, bool) {
	profiles := shellPathProfiles(home, shell, dir, goos)
	if len(profiles) == 0 {
		return shellProfile{}, false
	}
	if normalizeShellName(shell) != "" {
		return profiles[0], true
	}
	for _, profile := range profiles {
		if fileExists(profile.Path) {
			return profile, true
		}
	}
	return profiles[0], true
}

func shellPathBlockBounds(lines []string) (start int, end int) {
	start = -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if trimmed == shellPathBlockBegin {
				start = i
			}
			continue
		}
		if trimmed == shellPathBlockEnd {
			return start, i
		}
	}
	return start, -1
}

func upsertShellPathBlock(content, body string) string {
	blockLines := []string{shellPathBlockBegin}
	blockLines = append(blockLines, strings.Split(body, "\n")...)
	blockLines = append(blockLines, shellPathBlockEnd)

	lines := strings.Split(content, "\n")
	start, end := shellPathBlockBounds(lines)
	switch {
	case start >= 0 && end > start:
		next := make([]string, 0, len(lines))
		next = append(next, lines[:start]...)
		next = append(next, blockLines...)
		next = append(next, lines[end+1:]...)
		return strings.Join(next, "\n")
	case start >= 0:

		return strings.Join(append(lines, blockLines...), "\n")
	default:
		if strings.TrimSpace(content) == "" {
			return strings.Join(blockLines, "\n") + "\n"
		}
		trimmed := strings.TrimRight(content, "\n")
		return trimmed + "\n\n" + strings.Join(blockLines, "\n") + "\n"
	}
}

func removeShellPathBlock(content string) string {
	lines := strings.Split(content, "\n")
	start, end := shellPathBlockBounds(lines)
	if start < 0 || end <= start {
		return content
	}
	next := make([]string, 0, len(lines))
	next = append(next, lines[:start]...)
	next = append(next, lines[end+1:]...)
	trimmed := strings.TrimRight(strings.Join(next, "\n"), "\n")
	if strings.TrimSpace(trimmed) == "" {
		return ""
	}
	return trimmed + "\n"
}

func writeShellPathBlock(path, body string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(body) == "" {
		return errors.New("empty shell profile path or body")
	}
	info, statErr := os.Stat(path)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	existing, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	updated := upsertShellPathBlock(string(existing), body)
	if updated == string(existing) {
		return nil
	}
	perm := os.FileMode(0o644)
	if statErr == nil {

		perm = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), perm)
}

func removeShellPathBlockFrom(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		return readErr
	}
	updated := removeShellPathBlock(string(existing))
	if updated == string(existing) {
		return nil
	}
	if updated == "" {

		return os.WriteFile(path, nil, 0o644)
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

func shellPathBlockPresent(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	start, end := shellPathBlockBounds(strings.Split(string(existing), "\n"))
	return start >= 0 && end > start
}

func writeShellSearchPath(dir, home, shell, goos string) error {
	profile, ok := activeShellProfile(home, shell, dir, goos)
	if !ok {
		return errors.New("no shell profile available")
	}
	return writeShellPathBlock(profile.Path, profile.Body)
}

func removeShellSearchPath(dir, home, shell, goos string) error {
	var firstErr error
	for _, profile := range shellPathProfiles(home, shell, dir, goos) {
		if err := removeShellPathBlockFrom(profile.Path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func shellSearchPathFile(dir, home, shell, goos string) string {
	for _, profile := range shellPathProfiles(home, shell, dir, goos) {
		if shellPathBlockPresent(profile.Path) {
			return profile.Path
		}
	}
	return ""
}
