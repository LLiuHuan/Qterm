package session

import (
	"context"
	"io/fs"
	"time"

	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type SessionConnection interface {
	Wait() error
	Close() error
}

type SessionTerminal interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(columns int, rows int) error
	Wait() error
	Close() error
}

type terminalExitStatus interface {
	ExitStatus() (code int, known bool)
}

type TerminalRecorder interface {
	Output([]byte) error

	Input([]byte) error
	Resize(columns, rows int) error
	Close() error
}

type terminalEncodingProvider interface {
	terminalCharacterEncoding() string
}

type terminalCapableConnection interface {
	OpenTerminal(ctx context.Context, columns int, rows int) (SessionTerminal, error)
}

type latencyCapableConnection interface {
	MeasureLatency(context.Context) (time.Duration, error)
}

type latencyEnabledConnection interface {
	LatencyEnabled() bool
}
type resourceCapableConnection interface {
	ReadResources(context.Context) (model.ResourceSnapshot, error)
}

type SessionFileSystem interface {
	ReadDirContext(context.Context, string) ([]fs.FileInfo, error)
	StatContext(context.Context, string) (fs.FileInfo, error)
	Close() error
}

type SessionTransferFileSystem interface {
	SessionFileSystem
	UploadFileContext(context.Context, string, string, int64, bool, func(int64) error) error
	DownloadFileContext(context.Context, string, string, int64, bool, func(int64) error) error
}

type ResumableUploadFileSystem interface {
	UploadFileParallelResumable(
		ctx context.Context,
		localPath string,
		remotePath string,
		expectedSize int64,
		overwrite bool,
		progress func(int64) error,
		resumeKey string,
	) error

	DropResumeState(resumeKey string) string
}

type ParallelCapableFileSystem interface {
	UploadFileParallel(
		ctx context.Context,
		localPath string,
		remotePath string,
		expectedSize int64,
		overwrite bool,
		preserveDestination bool,
		progress func(int64) error,
	) error
	DownloadFileParallel(
		ctx context.Context,
		remotePath string,
		localPath string,
		expectedSize int64,
		overwrite bool,
		progress func(int64) error,
	) error

	TemporaryUploadPath(err error) string
}

type ConnectionStatusFileSystem interface {
	TransferConnectionStatus() (connections int, desired int, blockedReason string)

	TransferExpansionCounters() (expanded int, degraded int)
}

type auxiliaryProvisionerInstaller interface {
	SetAuxiliaryProvisioner(provisioner sshclient.ConnectionProvisioner)
}

type fileSystemConnectionPlanner interface {
	SetFileSystemConnections(count int)
}

type transferFileSystemOpener interface {
	OpenFileSystemForConnections(context.Context, int) (SessionFileSystem, error)
}

type fileSystemCapableConnection interface {
	OpenFileSystem(context.Context) (SessionFileSystem, error)
}

type DockerCommandRunner interface {
	RunDockerCommand(ctx context.Context, args []string) ([]byte, int, error)
}

type dockerCommandRunner = DockerCommandRunner

var _ dockerCommandRunner = (*sshclient.Connection)(nil)

type SessionConnector interface {
	Connect(
		ctx context.Context,
		sessionID string,
		hostID string,
		reportState func(model.SessionState) error,
	) (SessionConnection, error)
}

type sessionKindProvider interface {
	SessionKind() model.SessionKind
}

type failureSource interface {
	Failure(sessionID string) (string, bool)
}
