package vault

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const maxPasswordHintSize = 512

var (
	ErrInvalidPasswordHint = errors.New("invalid vault password hint")
)

func NormalizePasswordHint(hint string) (string, error) {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return "", nil
	}
	if len(hint) > maxPasswordHintSize || !utf8.ValidString(hint) || strings.IndexByte(hint, 0) >= 0 {
		return "", ErrInvalidPasswordHint
	}
	return hint, nil
}

func LoadPasswordHint(path string) (string, error) {
	if err := validatePasswordKeyPath(path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect password hint: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxPasswordHintSize {
		return "", ErrInvalidPasswordHint
	}
	if err := protectLocalPath(path, false); err != nil {
		return "", fmt.Errorf("protect password hint: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open password hint: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return "", ErrInvalidPasswordHint
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return "", fmt.Errorf("protect password hint: %w", err)
		}
	}
	value, err := io.ReadAll(io.LimitReader(file, maxPasswordHintSize+1))
	if err != nil {
		return "", fmt.Errorf("read password hint: %w", err)
	}
	if len(value) > maxPasswordHintSize {
		return "", ErrInvalidPasswordHint
	}
	hint, err := NormalizePasswordHint(string(value))
	if err != nil {
		return "", err
	}
	return hint, nil
}

func SavePasswordHint(path, hint string) error {
	if err := validatePasswordKeyPath(path); err != nil {
		return err
	}
	hint, err := NormalizePasswordHint(hint)
	if err != nil {
		return err
	}
	if hint == "" {

		info, statErr := os.Lstat(path)
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("inspect password hint before removal: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			return ErrInvalidPasswordHint
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove password hint: %w", err)
		}
		return syncPasswordKeyDirectory(filepath.Dir(path))
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() {
			return ErrInvalidPasswordHint
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect password hint before replacement: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create password hint directory: %w", err)
	}
	if err := protectLocalPath(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("protect password hint directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("protect password hint directory: %w", err)
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".qterm-password-hint-*")
	if err != nil {
		return fmt.Errorf("create temporary password hint: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := protectLocalPath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("protect temporary password hint: %w", err)
	}
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if runtime.GOOS != "windows" {
		if err := temporary.Chmod(0o600); err != nil {
			return fmt.Errorf("protect temporary password hint: %w", err)
		}
	}
	if _, err := temporary.WriteString(hint); err != nil {
		return fmt.Errorf("write temporary password hint: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary password hint: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary password hint: %w", err)
	}
	if err := replaceFileAtomically(temporaryPath, path); err != nil {
		return fmt.Errorf("commit password hint: %w", err)
	}
	keepTemporary = false
	if err := protectLocalPath(path, false); err != nil {
		return fmt.Errorf("protect password hint: %w", err)
	}
	return syncPasswordKeyDirectory(filepath.Dir(path))
}
