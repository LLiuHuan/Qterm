package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	dockerCommandStdoutLimit = 8 << 20

	dockerCommandStderrLimit = 4 * 1024

	dockerCommandMaxDuration = 2 * time.Minute

	dockerUnavailableExitCode = 127

	dockerArgumentCountLimit = 128
	dockerArgumentSizeLimit  = 1024
)

var (
	ErrDockerUnavailable = errors.New("remote docker CLI is unavailable")

	ErrDockerPermissionDenied = errors.New("remote docker socket permission denied")

	ErrDockerDaemonUnavailable = errors.New("remote docker daemon is unavailable")

	ErrDockerOutputTooLarge = errors.New("remote docker command output exceeds the limit")

	ErrInvalidDockerArguments = errors.New("invalid docker command arguments")
)

type DockerCommandError struct {
	ExitCode int
}

func (e *DockerCommandError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("remote docker command failed with exit status %d", e.ExitCode)
}

func (c *Connection) RunDockerCommand(ctx context.Context, args []string) ([]byte, int, error) {
	if ctx == nil {
		return nil, -1, errors.New("docker command context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, -1, err
	}

	if err := validateDockerArguments(args); err != nil {
		return nil, -1, err
	}
	if c == nil || c.client == nil {
		return nil, -1, errors.New("SSH connection is not ready")
	}
	command := buildDockerCommand(args)
	commandCtx, cancel := context.WithTimeout(ctx, dockerCommandMaxDuration)
	defer cancel()

	session, err := openCommandChannel(commandCtx, c.client)
	if err != nil {
		return nil, -1, err
	}
	defer func() { _ = session.Close() }()

	stdout := &boundedCommandOutput{limit: dockerCommandStdoutLimit}

	stdout.onOverflow = func() { _ = session.Close() }
	stderr := &boundedStderr{limit: dockerCommandStderrLimit}
	session.Stdout = stdout
	session.Stderr = stderr
	stop := context.AfterFunc(commandCtx, func() { _ = session.Close() })
	defer stop()

	runErr := session.Run(command)
	if runErr == nil {

		if stdout.Overflowed() {
			return nil, -1, ErrDockerOutputTooLarge
		}
		return stdout.Bytes(), 0, nil
	}
	if ctxErr := commandCtx.Err(); ctxErr != nil {

		return nil, -1, ctxErr
	}
	if stdout.Overflowed() {
		return nil, -1, ErrDockerOutputTooLarge
	}
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		exitCode := exitErr.ExitStatus()
		return stdout.Bytes(), exitCode, classifyDockerFailure(exitCode, stderr.String())
	}

	return nil, -1, runErr
}

func validateDockerArguments(args []string) error {
	if len(args) == 0 || len(args) > dockerArgumentCountLimit {
		return ErrInvalidDockerArguments
	}
	for _, arg := range args {
		if arg == "" || len(arg) > dockerArgumentSizeLimit {
			return ErrInvalidDockerArguments
		}

		if strings.ContainsAny(arg, "\x00\r\n") {
			return ErrInvalidDockerArguments
		}
	}
	if _, allowed := dockerAllowedSubcommands[args[0]]; !allowed {
		return ErrInvalidDockerArguments
	}
	return validateDockerArgumentFamilies(args)
}

var dockerAllowedSubcommands = map[string]struct{}{
	"version": {},
	"compose": {},
	"ps":      {},
	"inspect": {},
	"logs":    {},
	"start":   {},
	"stop":    {},
	"restart": {},
	"rm":      {},
	"images":  {},
	"image":   {},
	"volume":  {},
	"network": {},

	"rmi": {},
}

const dockerVolumeNamePattern = `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,254}$`

const dockerComposeProjectPattern = `^[a-z0-9][a-z0-9_-]*$`

const dockerNetworkIDPattern = `^[a-f0-9]{12,64}$`

const (
	dockerImageDigestRefPattern = `^sha256:[a-f0-9]{64}$`
	dockerImageShortRefPattern  = `^[a-f0-9]{12,64}$`
	dockerImageTagRefPattern    = `^[a-z0-9][a-z0-9._:/-]*:[a-zA-Z0-9_.-]{1,128}$`

	dockerImageRefSizeLimit = 255
)

var dockerComposeProjectRegexp = regexp.MustCompile(dockerComposeProjectPattern)

var dockerVolumeNameRegexp = regexp.MustCompile(dockerVolumeNamePattern)

var dockerNetworkIDRegexp = regexp.MustCompile(dockerNetworkIDPattern)

var (
	dockerImageDigestRefRegexp = regexp.MustCompile(dockerImageDigestRefPattern)
	dockerImageShortRefRegexp  = regexp.MustCompile(dockerImageShortRefPattern)
	dockerImageTagRefRegexp    = regexp.MustCompile(dockerImageTagRefPattern)
)

func dockerImageRefValid(ref string) bool {
	if len(ref) == 0 || len(ref) > dockerImageRefSizeLimit {
		return false
	}
	if dockerImageDigestRefRegexp.MatchString(ref) || dockerImageShortRefRegexp.MatchString(ref) {
		return true
	}
	if strings.HasPrefix(ref, "sha256:") {
		return false
	}

	return dockerImageTagRefRegexp.MatchString(ref) && !strings.Contains(ref, "..")
}

type dockerArgumentRule struct {
	tokens []string

	volumeNameIndex int

	freeformIndex int

	networkIDIndex int

	composeProjectIndex int
	composeFileIndex    int

	imageRefIndex int
}

var dockerArgumentRules = []dockerArgumentRule{
	{tokens: []string{"image", "prune", "-f"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"image", "prune", "-a", "-f"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"volume", "rm", ""}, volumeNameIndex: 2, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"volume", "ls", "--format", ""}, volumeNameIndex: -1, freeformIndex: 3,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},

	{tokens: []string{"image", "inspect", ""}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: 2},
	{tokens: []string{"volume", "inspect", ""}, volumeNameIndex: 2, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"network", "inspect", ""}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: 2, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},

	{tokens: []string{"rmi", ""}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: 1},

	{tokens: []string{"compose", "version", "--short"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"compose", "-p", "", "-f", "", "up", "-d"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: 2, composeFileIndex: 4, imageRefIndex: -1},
	{tokens: []string{"compose", "-p", "", "-f", "", "up", "-d", "--force-recreate"}, volumeNameIndex: -1,
		freeformIndex: -1, networkIDIndex: -1, composeProjectIndex: 2, composeFileIndex: 4, imageRefIndex: -1},
	{tokens: []string{"compose", "-p", "", "-f", "", "stop"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: 2, composeFileIndex: 4, imageRefIndex: -1},

	{tokens: []string{"compose", "-p", "", "-f", "", "down"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: 2, composeFileIndex: 4, imageRefIndex: -1},

	{tokens: []string{"network", "ls", "--no-trunc", "--format", ""}, volumeNameIndex: -1, freeformIndex: 4,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"network", "rm", ""}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: 2, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
	{tokens: []string{"network", "prune", "-f"}, volumeNameIndex: -1, freeformIndex: -1,
		networkIDIndex: -1, composeProjectIndex: -1, composeFileIndex: -1, imageRefIndex: -1},
}

func validateDockerArgumentFamilies(args []string) error {
	switch args[0] {
	case "image", "volume", "compose", "network", "rmi":
		for _, rule := range dockerArgumentRules {
			if matchesDockerArgumentRule(rule, args) {
				return nil
			}
		}
		return ErrInvalidDockerArguments
	default:
		return nil
	}
}

func matchesDockerArgumentRule(rule dockerArgumentRule, args []string) bool {
	if len(args) != len(rule.tokens) {
		return false
	}
	for index, token := range rule.tokens {
		if index == rule.volumeNameIndex {
			if !dockerVolumeNameRegexp.MatchString(args[index]) {
				return false
			}
			continue
		}
		if index == rule.freeformIndex {
			if strings.TrimSpace(args[index]) == "" {
				return false
			}
			continue
		}
		if index == rule.networkIDIndex {
			if !dockerNetworkIDRegexp.MatchString(args[index]) {
				return false
			}
			continue
		}
		if index == rule.composeProjectIndex {
			if !dockerComposeProjectRegexp.MatchString(args[index]) {
				return false
			}
			continue
		}
		if index == rule.composeFileIndex {

			if !strings.HasPrefix(args[index], "/") {
				return false
			}
			continue
		}
		if index == rule.imageRefIndex {
			if !dockerImageRefValid(args[index]) {
				return false
			}
			continue
		}
		if args[index] != token {
			return false
		}
	}
	return true
}

func buildDockerCommand(args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, "docker")
	for _, arg := range args {
		parts = append(parts, quotePOSIXShellArgument(arg))
	}
	return strings.Join(parts, " ")
}

func classifyDockerFailure(exitCode int, stderr string) error {
	lowered := strings.ToLower(stderr)
	if exitCode == dockerUnavailableExitCode || dockerCLIMissing(lowered) {
		return ErrDockerUnavailable
	}
	if strings.Contains(lowered, "permission denied") {
		return ErrDockerPermissionDenied
	}
	for _, marker := range dockerDaemonUnavailableMarkers {
		if strings.Contains(lowered, marker) {
			return ErrDockerDaemonUnavailable
		}
	}
	return &DockerCommandError{ExitCode: exitCode}
}

var dockerCLIMissingMarkers = []string{
	"docker: not found",
	"docker: command not found",
	"docker: no such file or directory",
}

func dockerCLIMissing(loweredStderr string) bool {
	for _, marker := range dockerCLIMissingMarkers {
		if strings.Contains(loweredStderr, marker) {
			return true
		}
	}
	return false
}

var dockerDaemonUnavailableMarkers = []string{
	"cannot connect to the docker daemon",
	"is the docker daemon running",
	"error during connect",
	"docker daemon is not running",
}

var errDockerOutputOverflow = errors.New("docker command output overflow")

type boundedCommandOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	overflow bool

	onOverflow func()
}

func (b *boundedCommandOutput) Write(value []byte) (int, error) {
	b.mu.Lock()
	if b.overflow {
		b.mu.Unlock()
		return 0, errDockerOutputOverflow
	}
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 || len(value) > remaining {
		b.overflow = true
		onOverflow := b.onOverflow
		b.mu.Unlock()
		if onOverflow != nil {
			onOverflow()
		}
		return 0, errDockerOutputOverflow
	}
	count, err := b.buffer.Write(value)
	b.mu.Unlock()
	return count, err
}

func (b *boundedCommandOutput) Overflowed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overflow
}

func (b *boundedCommandOutput) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buffer.Len() == 0 {
		return nil
	}
	return append([]byte(nil), b.buffer.Bytes()...)
}
