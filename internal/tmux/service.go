package tmux

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"changeme/internal/errno"
)

type CommandResult struct {
	Output   string
	ExitCode int
}

type CommandRunner interface {
	RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (CommandResult, error)
}

type Session struct {
	Name string `json:"name"`

	Windows int `json:"windows"`

	CreatedAt int64 `json:"createdAt"`

	Attached bool `json:"attached"`
}

const (
	commandTimeout = 10 * time.Second

	maxSessions = 200

	maxNameLength = 64
)

type Service struct {
	runner CommandRunner
}

func NewService(runner CommandRunner) *Service {
	return &Service{runner: runner}
}

func (s *Service) SetCommandRunner(runner CommandRunner) {
	if s == nil {
		return
	}
	s.runner = runner
}

var ErrUnavailable = errno.New(errno.CodeConnFailed, "tmux.unavailable", "")

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLength {
		return "", errno.New(errno.CodeInvalidArg, "tmux.invalid_name", "")
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_':
		default:
			return "", errno.New(errno.CodeInvalidArg, "tmux.invalid_name", "")
		}
	}
	return name, nil
}

func ValidateTargetName(name string) (string, error) {
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) {
		return "", errno.New(errno.CodeInvalidArg, "tmux.invalid_name", "")
	}
	for _, char := range name {
		if unicode.IsControl(char) || char == '\u2028' || char == '\u2029' {
			return "", errno.New(errno.CodeInvalidArg, "tmux.invalid_name", "")
		}
	}
	return name, nil
}

func (s *Service) List(ctx context.Context, sessionID string) ([]Session, error) {
	output, exitCode, err := s.exec(ctx, sessionID, listCommand)
	if err != nil {
		return nil, err
	}
	if exitCode != 0 {

		if isNoServer(output) {
			return []Session{}, nil
		}
		return nil, errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
	}
	return parseList(output), nil
}

func (s *Service) Create(ctx context.Context, sessionID, name string) error {
	normalized, err := ValidateName(name)
	if err != nil {
		return err
	}
	output, exitCode, err := s.exec(ctx, sessionID, buildNewCommand(normalized))
	if err != nil {
		return err
	}
	if exitCode == 0 {
		return nil
	}

	if strings.Contains(strings.ToLower(output), "duplicate session") {
		return errno.New(errno.CodeInvalidArg, "tmux.session_exists", "")
	}
	return errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
}

func (s *Service) Kill(ctx context.Context, sessionID, name string) error {
	normalized, err := ValidateTargetName(name)
	if err != nil {
		return err
	}
	output, exitCode, err := s.exec(ctx, sessionID, buildKillCommand(normalized))
	if err != nil {
		return err
	}
	if exitCode == 0 {
		return nil
	}

	lowered := strings.ToLower(output)
	if strings.Contains(lowered, "can't find session") || isNoServer(output) {
		return errno.New(errno.CodeNotFound, "tmux.session_not_found", "")
	}
	return errno.New(errno.CodeConnFailed, "tmux.command_failed", "exit "+strconv.Itoa(exitCode))
}

func (s *Service) exec(ctx context.Context, sessionID, command string) (string, int, error) {
	if s == nil || s.runner == nil {
		return "", 0, ErrUnavailable
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", 0, errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	result, err := s.runner.RunAIBackgroundCommand(ctx, sessionID, command)
	if err != nil {
		return "", 0, err
	}

	if result.ExitCode != 0 && (result.ExitCode == 127 || isMissingTmuxText(result.Output)) {
		return "", result.ExitCode, ErrUnavailable
	}
	return result.Output, result.ExitCode, nil
}

func isMissingTmuxText(output string) bool {
	lowered := strings.ToLower(output)
	return strings.Contains(lowered, "tmux: not found") || strings.Contains(lowered, "tmux: command not found")
}

func isNoServer(output string) bool {
	lowered := strings.ToLower(output)
	return strings.Contains(lowered, "no server running") || strings.Contains(lowered, "no sessions") ||
		isNoServerConnecting(lowered)
}

func isNoServerConnecting(lowered string) bool {
	if !strings.Contains(lowered, "error connecting to") {
		return false
	}
	return strings.Contains(lowered, "no such file or directory") ||
		strings.Contains(lowered, "connection refused")
}

const listCommand = `tmux list-sessions -F '#{session_name}|#{session_windows}|#{session_created}|#{session_attached}' 2>&1`

func parseList(output string) []Session {
	sessions := make([]Session, 0, 8)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(strings.TrimSuffix(line, "\r"), "|")
		if len(fields) < 4 {
			continue
		}

		name := strings.Join(fields[:len(fields)-3], "|")
		if _, err := ValidateTargetName(name); err != nil {
			continue
		}
		windows, windowsErr := strconv.Atoi(fields[len(fields)-3])
		created, createdErr := strconv.ParseInt(fields[len(fields)-2], 10, 64)
		attached, attachedErr := strconv.Atoi(fields[len(fields)-1])
		if windowsErr != nil || createdErr != nil || attachedErr != nil || windows < 1 || created < 0 || attached < 0 {
			continue
		}
		sessions = append(sessions, Session{
			Name:      name,
			Windows:   windows,
			CreatedAt: created,
			Attached:  attached > 0,
		})
		if len(sessions) >= maxSessions {
			break
		}
	}
	return sessions
}

func buildNewCommand(name string) string {
	return "tmux new-session -d -s " + shellQuote(name)
}

func buildKillCommand(name string) string {

	return "tmux kill-session -t " + shellQuote("="+name)
}

func BuildAttachCommand(name string) string {

	target := shellQuote("=" + name)
	command := `if [ -n "${TMUX:-}" ]; then tmux switch-client -t ` + target + `; else tmux attach -t ` + target + `; fi`

	return "sh -c " + terminalShellWord(command)
}

func terminalShellWord(value string) string {
	var quoted strings.Builder
	for _, char := range value {
		if strings.ContainsRune(" \\\"'`$;&|<>()[]{}*?!#~", char) {
			quoted.WriteByte('\\')
		}
		quoted.WriteRune(char)
	}
	return quoted.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
