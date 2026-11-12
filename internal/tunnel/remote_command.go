package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"changeme/internal/model"
)

const (
	remoteArchiveExtractTimeout = 10 * time.Minute

	remoteTarProbeTimeout = 30 * time.Second

	remoteArchiveCreateTimeout = 10 * time.Minute

	remoteRemoveTimeout = 10 * time.Minute

	remoteCommandStderrLimit = 4 * 1024

	shellNotFoundExitCode = 127

	remoteCommandTarExtract = "tar-extract"
	remoteCommandTarCreate  = "tar-create"
	remoteCommandTarProbe   = "tar-probe"
	remoteCommandRemoveTree = "rm-tree"
)

var (
	ErrRemoteTarUnavailable = errors.New("remote tar is unavailable")

	ErrRemoteRemoveUnavailable = errors.New("remote rm is unavailable")
)

type RemoteCommandError struct {
	Operation string

	ExitCode int
}

func (e *RemoteCommandError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("remote %s failed with exit status %d", e.Operation, e.ExitCode)
}

func (c *Connection) ExtractRemoteArchive(
	ctx context.Context,
	archivePath string,
	targetDirectory string,
) error {

	command, err := buildTarExtractCommand(archivePath, targetDirectory)
	if err != nil {
		return err
	}
	if c == nil || c.client == nil {
		return errors.New("SSH connection is not ready")
	}
	return runFixedRemoteCommand(
		ctx,
		sshCommandExecutor{client: c.client},
		command,
		remoteArchiveExtractTimeout,
		remoteCommandTarExtract,
	)
}

func (c *Connection) CreateRemoteArchive(
	ctx context.Context,
	archivePath string,
	sourceDirectory string,
) error {

	command, err := buildTarCreateCommand(archivePath, sourceDirectory)
	if err != nil {
		return err
	}
	if c == nil || c.client == nil {
		return errors.New("SSH connection is not ready")
	}
	return runFixedRemoteCommand(
		ctx,
		sshCommandExecutor{client: c.client},
		command,
		remoteArchiveCreateTimeout,
		remoteCommandTarCreate,
	)
}

func (c *Connection) ProbeRemoteTar(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("SSH connection is not ready")
	}
	return runFixedRemoteCommand(
		ctx,
		sshCommandExecutor{client: c.client},
		buildTarProbeCommand(),
		remoteTarProbeTimeout,
		remoteCommandTarProbe,
	)
}

func (c *Connection) RemoveRemoteTree(ctx context.Context, remotePath string) error {

	command, err := buildRemoveTreeCommand(remotePath)
	if err != nil {
		return err
	}
	if c == nil || c.client == nil {
		return errors.New("SSH connection is not ready")
	}
	return runFixedRemoteCommandWithAvailability(
		ctx,
		sshCommandExecutor{client: c.client},
		command,
		remoteRemoveTimeout,
		remoteCommandRemoveTree,
		remoteCommandAvailability{
			unavailable: ErrRemoteRemoveUnavailable,
			notFound:    rmNotFound,
		},
	)
}

func buildTarExtractCommand(archivePath, targetDirectory string) (string, error) {
	archive, err := model.NormalizeRemotePath(archivePath)
	if err != nil {
		return "", err
	}
	target, err := model.NormalizeRemoteDirectory(targetDirectory)
	if err != nil {
		return "", err
	}
	return "tar -xzf " + quotePOSIXShellArgument(archive) + " -C " + quotePOSIXShellArgument(target), nil
}

func buildTarCreateCommand(archivePath, sourceDirectory string) (string, error) {
	archive, err := model.NormalizeRemotePath(archivePath)
	if err != nil {
		return "", err
	}
	source, err := model.NormalizeRemoteDirectory(sourceDirectory)
	if err != nil {
		return "", err
	}
	if source == "/" {
		return "", model.ErrInvalidRemotePath
	}
	return "tar -czf " + quotePOSIXShellArgument(archive) +
		" -C " + quotePOSIXShellArgument(source) + " .", nil
}

func buildTarProbeCommand() string {
	return "tar --version"
}

func buildRemoveTreeCommand(remotePath string) (string, error) {

	raw := remotePath
	normalized, err := model.NormalizeRemotePath(raw)
	if err != nil {

		return "", err
	}

	if normalized == "" {
		return "", model.ErrInvalidRemotePath
	}

	if normalized == "/" {
		return "", model.ErrInvalidRemotePath
	}

	for _, segment := range strings.Split(raw, "/") {
		if segment == "." || segment == ".." {
			return "", model.ErrInvalidRemotePath
		}
	}

	if strings.HasPrefix(raw, "~") {
		return "", model.ErrInvalidRemotePath
	}

	if strings.ContainsAny(raw, "\x00\n") {
		return "", model.ErrInvalidRemotePath
	}

	return "rm -rf -- " + quotePOSIXShellArgument(normalized), nil
}

func quotePOSIXShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

type remoteCommandExecutor interface {
	execute(ctx context.Context, command string, stderr io.Writer) (int, error)
}

type sshCommandExecutor struct {
	client *ssh.Client
}

var _ remoteCommandExecutor = sshCommandExecutor{}

func (e sshCommandExecutor) execute(ctx context.Context, command string, stderr io.Writer) (int, error) {
	if e.client == nil {
		return -1, errors.New("SSH connection is not ready")
	}
	session, err := openCommandChannel(ctx, e.client)
	if err != nil {
		return -1, err
	}
	defer func() { _ = session.Close() }()
	session.Stdout = io.Discard
	session.Stderr = stderr
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	runErr := session.Run(command)
	if ctxErr := ctx.Err(); ctxErr != nil {

		return -1, ctxErr
	}
	if runErr == nil {
		return 0, nil
	}
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitStatus(), nil
	}

	return -1, runErr
}

func openCommandChannel(ctx context.Context, client *ssh.Client) (*ssh.Session, error) {
	type openResult struct {
		session *ssh.Session
		err     error
	}
	opened := make(chan openResult)
	go func() {
		session, err := client.NewSession()
		select {
		case opened <- openResult{session: session, err: err}:
		case <-ctx.Done():
			if session != nil {
				_ = session.Close()
			}
		}
	}()
	select {
	case result := <-opened:
		if err := ctx.Err(); err != nil {
			if result.session != nil {
				_ = result.session.Close()
			}
			return nil, err
		}
		if result.err != nil {
			return nil, result.err
		}
		return result.session, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type remoteCommandAvailability struct {
	unavailable error

	notFound func(stderr string) bool
}

var tarAvailability = remoteCommandAvailability{
	unavailable: ErrRemoteTarUnavailable,
	notFound:    tarNotFound,
}

func runFixedRemoteCommand(
	ctx context.Context,
	executor remoteCommandExecutor,
	command string,
	timeout time.Duration,
	operation string,
) error {
	return runFixedRemoteCommandWithAvailability(ctx, executor, command, timeout, operation, tarAvailability)
}

func runFixedRemoteCommandWithAvailability(
	ctx context.Context,
	executor remoteCommandExecutor,
	command string,
	timeout time.Duration,
	operation string,
	availability remoteCommandAvailability,
) error {
	if ctx == nil {
		return errors.New("remote command context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if executor == nil {
		return errors.New("remote command executor is not ready")
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stderr := boundedStderr{limit: remoteCommandStderrLimit}
	exitCode, err := executor.execute(commandCtx, command, &stderr)
	if err == nil && exitCode == 0 {

		return nil
	}
	if ctxErr := commandCtx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {

		return fmt.Errorf("run remote %s: %w", operation, err)
	}
	if exitCode < 0 {

		return fmt.Errorf("run remote %s: exit status unavailable", operation)
	}
	if exitCode == shellNotFoundExitCode || (availability.notFound != nil && availability.notFound(stderr.String())) {
		return availability.unavailable
	}
	return &RemoteCommandError{Operation: operation, ExitCode: exitCode}
}

var tarNotFoundMarkers = []string{
	"tar: not found",
	"tar: command not found",
	"tar: no such file or directory",
}

func tarNotFound(stderr string) bool {
	lowered := strings.ToLower(stderr)
	for _, marker := range tarNotFoundMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

var rmNotFoundSuffixes = []string{
	"rm: not found",
	"rm: command not found",
	"command not found: rm",
}

func rmNotFound(stderr string) bool {
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		for _, suffix := range rmNotFoundSuffixes {
			if strings.HasSuffix(trimmed, suffix) {
				return true
			}
		}
	}
	return false
}

type boundedStderr struct {
	data  []byte
	limit int
}

func (b *boundedStderr) Write(value []byte) (int, error) {
	if b.limit <= 0 {
		return len(value), nil
	}
	if len(value) >= b.limit {
		b.data = append(b.data[:0], value[len(value)-b.limit:]...)
		return len(value), nil
	}
	b.data = append(b.data, value...)
	if over := len(b.data) - b.limit; over > 0 {
		b.data = append(b.data[:0], b.data[over:]...)
	}
	return len(value), nil
}

func (b *boundedStderr) String() string {
	return string(b.data)
}
