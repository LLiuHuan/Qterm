package tmux

import (
	"context"
	"strconv"
	"strings"

	"changeme/internal/errno"
)

func (s *Service) Version(ctx context.Context, sessionID string) (string, error) {
	output, code, err := s.exec(ctx, sessionID, "tmux -V 2>&1")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(code))
	}
	version := strings.TrimSpace(output)

	if len(version) > 128 || !strings.HasPrefix(version, "tmux ") || len(version) <= len("tmux ") {
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "")
	}

	for strings.HasPrefix(version, "tmux ") {
		rest := strings.TrimSpace(strings.TrimPrefix(version, "tmux "))

		if rest == "" || rest == version {
			break
		}
		version = rest
	}
	if version == "" || version == "tmux" || strings.HasPrefix(version, "tmux ") {
		return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "")
	}
	for _, c := range version {
		if c < 0x20 || c > 0x7e {
			return "", errno.New(errno.CodeConnFailed, "tmux.command_failed", "")
		}
	}
	return version, nil
}

func BuildTrackedAttachCommand(name, token string) (string, error) {
	if _, err := ValidateTargetName(name); err != nil {
		return "", err
	}
	if token == "" {
		return BuildAttachCommand(name), nil
	}
	if !isTrackingToken(token) {
		return "", errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	target := shellQuote("=" + name)
	enter := `printf '\033]777;qterm-tmux;` + token + `;enter\007'`
	exit := `printf '\033]777;qterm-tmux;` + token + `;exit\007'`
	command := `if [ -n "${TMUX:-}" ]; then tmux switch-client -t ` + target +
		`; else trap ` + shellQuote(exit) + ` EXIT; ` + enter + `; tmux attach -t ` + target + `; fi`
	return "sh -c " + terminalShellWord(command), nil
}

func isTrackingToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, c := range token {
		if !((c >= 'a' && c <= 'f') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
