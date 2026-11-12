package cliapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const HandshakeVersion = 1

type Handshake struct {
	Version int    `json:"version"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
}

func WriteHandshake(path string, info Handshake) error {
	if path == "" {
		return errors.New("cliapi: handshake path is required")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("cliapi: create handshake directory: %w", err)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("cliapi: encode handshake: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("cliapi: write handshake: %w", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("cliapi: chmod handshake: %w", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cliapi: stat handshake: %w", err)
	}

	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0o600 {
		return fmt.Errorf("cliapi: handshake permission is %o, want 600", stat.Mode().Perm())
	}
	return nil
}

func RemoveHandshake(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cliapi: remove handshake: %w", err)
	}
	return nil
}

func ReadHandshake(path string) (Handshake, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Handshake{}, err
	}
	var info Handshake
	if err := json.Unmarshal(data, &info); err != nil {
		return Handshake{}, fmt.Errorf("cliapi: decode handshake: %w", err)
	}
	if info.Version != HandshakeVersion {
		return Handshake{}, fmt.Errorf("cliapi: handshake version %d is not supported", info.Version)
	}
	if info.Port <= 0 || info.Port > 65535 {
		return Handshake{}, errors.New("cliapi: handshake port is invalid")
	}
	if info.Token == "" {
		return Handshake{}, errors.New("cliapi: handshake token is empty")
	}
	return info, nil
}
