package tmux

import (
	"context"
	"strconv"
	"strings"

	"changeme/internal/errno"
)

func (s *Service) Rename(ctx context.Context, sessionID, name, newName string) error {
	target, err := ValidateTargetName(name)
	if err != nil {
		return err
	}
	normalized, err := ValidateName(newName)
	if err != nil {
		return err
	}

	command := "tmux rename-session -t " + shellQuote("="+target) + " -- " + shellQuote(normalized)
	output, exitCode, err := s.exec(ctx, sessionID, command)
	if err != nil {
		return err
	}
	if exitCode == 0 {
		return nil
	}
	lowered := strings.ToLower(output)
	if strings.Contains(lowered, "duplicate session") {
		return errno.New(errno.CodeInvalidArg, "tmux.session_exists", "")
	}
	if strings.Contains(lowered, "can't find session") || isNoServer(output) {
		return errno.New(errno.CodeNotFound, "tmux.session_not_found", "")
	}
	return errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
}

func (s *Service) DetachKeys(ctx context.Context, sessionID string) (string, error) {
	prefixBytes, err := s.prefixBytes(ctx, sessionID)
	if err != nil {
		return "", err
	}
	output, exitCode, err := s.exec(ctx, sessionID, "tmux list-keys -T prefix 2>&1")
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)

		if len(fields) != 5 || fields[0] != "bind-key" || fields[1] != "-T" || fields[2] != "prefix" || fields[4] != "detach-client" {
			continue
		}
		keyBytes, valid := tmuxKeyBytes(fields[3])
		keys := prefixBytes + keyBytes

		if valid && len(keyBytes) == 1 && keyBytes[0] >= 0x20 && keyBytes[0] <= 0x7e {
			return keys, nil
		}
	}
	return "", detachUnavailable()
}

func (s *Service) PrefixKeys(ctx context.Context, sessionID string) (string, error) {
	output, exitCode, err := s.exec(ctx, sessionID, "tmux show-options -g prefix 2>&1")
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		if isNoServer(output) {
			return "", errno.New(errno.CodeNotFound, "tmux.session_not_found", "")
		}
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
	}

	fields := strings.Fields(output)
	if len(fields) != 2 || fields[0] != "prefix" {
		return "", detachUnavailable()
	}
	return encodePrefix(fields[1])
}

func (s *Service) prefixBytes(ctx context.Context, sessionID string) (string, error) {
	output, exitCode, err := s.exec(ctx, sessionID, "tmux list-sessions -F '#{prefix}' 2>&1")
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		if isNoServer(output) {
			return "", errno.New(errno.CodeNotFound, "tmux.session_not_found", "")
		}
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
	}
	prefix := ""
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		key := strings.TrimSpace(line)
		if key == "" || (prefix != "" && prefix != key) {
			return "", detachUnavailable()
		}
		prefix = key
	}
	return encodePrefix(prefix)
}

func encodePrefix(key string) (string, error) {
	prefixBytes, ok := tmuxKeyBytes(key)
	if !ok || (prefixBytes != "\x01" && prefixBytes != "\x02") {

		return "", detachUnavailable()
	}
	return prefixBytes, nil
}

func detachUnavailable() error {
	return errno.New(errno.CodeInvalidArg, "tmux.detach_unavailable", "")
}

func tmuxKeyBytes(key string) (string, bool) {
	switch key {
	case "C-a":
		return "\x01", true
	case "C-b":
		return "\x02", true
	case "Space":
		return " ", true
	}
	if len(key) == 1 && key[0] >= 0x21 && key[0] <= 0x7e {
		return key, true
	}
	return "", false
}
