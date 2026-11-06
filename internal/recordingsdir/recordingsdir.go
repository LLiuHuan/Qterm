package recordingsdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func Default() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(base) == "" {
		return "", errors.New("user config directory is empty")
	}
	return filepath.Join(base, "Qterm", "recordings"), nil
}

func Effective(configured string) (string, error) {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" {
		return Default()
	}
	normalized, err := normalize(trimmed)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func Validate(path string) (string, error) {
	return normalize(path)
}

func normalize(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("recording directory is empty")
	}
	expanded, err := expandHome(trimmed)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		return "", errors.New("recording directory must be absolute")
	}
	cleaned := filepath.Clean(expanded)

	if cleaned == filepath.VolumeName(cleaned)+string(filepath.Separator) {
		return "", errors.New("recording directory must not be a filesystem root")
	}
	return cleaned, nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(filepath.Separator)) && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("user home directory is empty")
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), string(filepath.Separator))), nil
}
