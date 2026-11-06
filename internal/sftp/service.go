package sftp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"

	pkgsftp "github.com/pkg/sftp"
)

var ErrServiceNotReady = errors.New("SFTP service is not ready")

type FileDialogs interface {
	ChooseUploadFile() (string, error)
	ChooseDownloadFile(filename string) (string, error)
}

type Service struct {
	mu        sync.RWMutex
	manager   *sessionapp.SessionManager
	transfers *SFTPTransferManager
	local     *localDirectoryBrowser
	dialogs   FileDialogs
	drops     map[string]droppedFiles
	settings  Settings

	lastConnections     int
	lastDesired         int
	lastBlocked         string
	hasConnectionReport bool
}

func NewService() *Service {
	return &Service{local: newLocalDirectoryBrowser(), settings: DefaultSettings()}
}

func (s *Service) Start(manager *sessionapp.SessionManager) error {
	return s.StartWithConcurrency(manager, 3)
}

func (s *Service) StartWithConcurrency(manager *sessionapp.SessionManager, concurrency int) error {
	if manager == nil {
		return ErrServiceNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.transfers != nil {
		return errno.New(errno.CodeInvalidArg, "sftp.already_started", "")
	}
	if s.local == nil {
		s.local = newLocalDirectoryBrowser()
	}
	s.manager = manager
	executor := NewSessionSFTPTransferExecutor(manager)

	executor.SetConnectionReporter(s)
	s.transfers = NewSFTPTransferManager(executor, concurrency)
	settings := DefaultSettings()
	settings.MaxTransfers = concurrency
	s.settings = settings
	return nil
}

func (s *Service) ReportTransferConnections(connections int, desired int, blockedReason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.lastConnections = connections
	s.lastDesired = desired
	s.lastBlocked = blockedReason
	s.hasConnectionReport = true
	s.mu.Unlock()
}

func (s *Service) SetTransferConcurrency(concurrency int) error {
	if concurrency < 1 || concurrency > 16 {
		return errno.New(errno.CodeInvalidArg, "sftp.concurrency_invalid", "")
	}
	s.mu.RLock()
	transfers := s.transfers
	s.mu.RUnlock()
	if transfers == nil {
		return ErrServiceNotReady
	}
	return transfers.SetConcurrency(concurrency)
}

func (s *Service) Close() error {
	s.mu.Lock()
	transfers := s.transfers
	s.transfers = nil
	s.manager = nil
	if s.local != nil {
		s.local.clear()
	}
	s.drops = nil
	s.mu.Unlock()
	if transfers == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return transfers.Shutdown(ctx)
}

func (s *Service) SetDialogs(dialogs FileDialogs) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.dialogs = dialogs
	s.mu.Unlock()
}

type SFTPTransferSelectionResult struct {
	Canceled bool                `json:"canceled"`
	Transfer *model.SFTPTransfer `json:"transfer"`
}

func (s *Service) SelectSFTPUploadFile(
	ctx context.Context,
	sessionID string,
	remoteDirectory string,
) (SFTPTransferSelectionResult, error) {
	manager, transfers, dialogs, err := s.transferDependencies()
	if err != nil {
		return SFTPTransferSelectionResult{}, err
	}
	if dialogs == nil {
		return SFTPTransferSelectionResult{}, ErrServiceNotReady
	}
	remoteDirectory, err = model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	sessionInfo, err := manager.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil || sessionInfo.State != model.SessionStateConnected {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	localPath, err := dialogs.ChooseUploadFile()
	if err != nil {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeUnknown, "sftp.open_dialog_failed", "")
	}
	if localPath == "" {
		return SFTPTransferSelectionResult{Canceled: true}, nil
	}
	localPath, localName, size, err := InspectUploadSource(localPath)
	if err != nil {
		return SFTPTransferSelectionResult{}, err
	}
	remotePath, err := model.NormalizeRemotePath(path.Join(remoteDirectory, localName))
	if err != nil {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	transfer, err := transfers.Enqueue(ctx, SFTPTransferRequest{
		SessionID:  strings.TrimSpace(sessionID),
		Direction:  model.SFTPTransferUpload,
		LocalPath:  localPath,
		LocalName:  localName,
		RemotePath: remotePath,
		Total:      size,
	})
	if err != nil {
		return SFTPTransferSelectionResult{}, TransferServiceError(err)
	}
	return SFTPTransferSelectionResult{Transfer: &transfer}, nil
}

func (s *Service) SelectSFTPDownloadFile(
	ctx context.Context,
	sessionID string,
	remotePath string,
) (SFTPTransferSelectionResult, error) {
	manager, transfers, dialogs, err := s.transferDependencies()
	if err != nil {
		return SFTPTransferSelectionResult{}, err
	}
	if dialogs == nil {
		return SFTPTransferSelectionResult{}, ErrServiceNotReady
	}
	remotePath, err = model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	fileSystem, err := manager.FileSystem(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return SFTPTransferSelectionResult{}, ServiceError(err)
	}
	info, err := fileSystem.StatContext(ctx, remotePath)
	if err != nil {
		return SFTPTransferSelectionResult{}, RemotePathError(err, "sftp.stat_failed")
	}
	if !info.Mode().IsRegular() {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeInvalidArg, "sftp.download_regular_only", "")
	}
	localPath, err := dialogs.ChooseDownloadFile(path.Base(remotePath))
	if err != nil {
		return SFTPTransferSelectionResult{}, errno.New(errno.CodeUnknown, "sftp.save_dialog_failed", "")
	}
	if localPath == "" {
		return SFTPTransferSelectionResult{Canceled: true}, nil
	}
	localPath, err = ValidateDownloadDestination(localPath)
	if err != nil {
		return SFTPTransferSelectionResult{}, err
	}
	transfer, err := transfers.Enqueue(ctx, SFTPTransferRequest{
		SessionID:  strings.TrimSpace(sessionID),
		Direction:  model.SFTPTransferDownload,
		LocalPath:  localPath,
		LocalName:  filepath.Base(localPath),
		RemotePath: remotePath,
		Total:      info.Size(),
	})
	if err != nil {
		return SFTPTransferSelectionResult{}, TransferServiceError(err)
	}
	return SFTPTransferSelectionResult{Transfer: &transfer}, nil
}

func (s *Service) ListRemoteDirectory(ctx context.Context, sessionID, directory string) ([]model.RemoteFileEntry, error) {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return nil, err
	}
	fileSystem, err := manager.FileSystem(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, ServiceError(err)
	}

	if directory == "" {
		if resolver, ok := fileSystem.(interface {
			WorkingDirectory(context.Context) (string, error)
		}); ok {
			if home, homeErr := resolver.WorkingDirectory(ctx); homeErr == nil {
				if normalized, normalizeErr := model.NormalizeRemoteDirectory(home); normalizeErr == nil {
					directory = normalized
				}
			}
		}
	}
	directory, err = model.NormalizeRemoteDirectory(directory)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	entries, err := fileSystem.ReadDirContext(ctx, directory)
	if err != nil {
		return nil, ServiceError(err)
	}
	result, err := RemoteDirectoryEntries(directory, entries)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "sftp.filename_unsupported", "")
	}

	resolveSymlinkTargets(ctx, result, func(remotePath string) (fs.FileInfo, error) {
		return fileSystem.StatContext(ctx, remotePath)
	})
	return result, nil
}

const symlinkResolveConcurrency = 8

func resolveSymlinkTargets(
	ctx context.Context,
	entries []model.RemoteFileEntry,
	stat func(string) (fs.FileInfo, error),
) {
	if stat == nil {
		return
	}
	targets := make([]int, 0, len(entries))
	for index := range entries {
		if entries[index].Type == model.RemoteFileTypeSymlink {
			targets = append(targets, index)
		}
	}
	if len(targets) == 0 {
		return
	}
	workers := symlinkResolveConcurrency
	if workers > len(targets) {
		workers = len(targets)
	}
	jobs := make(chan int)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				info, err := stat(entries[index].Path)
				if err != nil || info == nil {
					continue
				}
				entries[index].TargetType = model.RemoteFileTypeFromMode(info.Mode())
			}
		}()
	}
	for _, index := range targets {
		if ctx.Err() != nil {
			break
		}
		jobs <- index
	}
	close(jobs)
	wait.Wait()
}

func (s *Service) ListSFTPTransfers(ctx context.Context) ([]model.SFTPTransfer, error) {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return nil, ServiceError(err)
	}
	result, err := transfers.List(ctx)
	if err != nil {
		return nil, TransferServiceError(err)
	}
	return result, nil
}

func (s *Service) ConfirmSFTPTransferOverwrite(
	ctx context.Context,
	id string,
) (model.SFTPTransfer, error) {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, ServiceError(err)
	}
	transfer, err := transfers.ConfirmOverwrite(ctx, id)
	if err != nil {
		return model.SFTPTransfer{}, TransferServiceError(err)
	}
	return transfer, nil
}

func (s *Service) RetrySFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, ServiceError(err)
	}
	transfer, err := transfers.Retry(ctx, id)
	if err != nil {
		return model.SFTPTransfer{}, TransferServiceError(err)
	}
	return transfer, nil
}

func (s *Service) CancelSFTPTransfer(ctx context.Context, id string) error {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return ServiceError(err)
	}
	if err := transfers.Cancel(ctx, id); err != nil {
		return TransferServiceError(err)
	}
	return nil
}

func (s *Service) PauseSFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, ServiceError(err)
	}
	transfer, pauseErr := transfers.Pause(ctx, id)
	if pauseErr != nil {
		return model.SFTPTransfer{}, TransferServiceError(pauseErr)
	}
	return transfer, nil
}

func (s *Service) ResumeSFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, ServiceError(err)
	}
	transfer, resumeErr := transfers.Resume(ctx, id)
	if resumeErr != nil {
		return model.SFTPTransfer{}, TransferServiceError(resumeErr)
	}
	return transfer, nil
}

func (s *Service) transferDependencies() (
	*sessionapp.SessionManager,
	*SFTPTransferManager,
	FileDialogs,
	error,
) {
	if s == nil {
		return nil, nil, nil, ErrServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manager == nil || s.transfers == nil {
		return nil, nil, nil, ErrServiceNotReady
	}
	return s.manager, s.transfers, s.dialogs, nil
}

func InspectUploadSource(localPath string) (string, string, int64, error) {

	localPath = filepath.Clean(strings.TrimSpace(localPath))
	if !filepath.IsAbs(localPath) {
		return "", "", 0, errno.New(errno.CodeInvalidArg, "sftp.local_path_invalid", "")
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return "", "", 0, errno.New(errno.CodeStoreFailed, "sftp.source_failed", "")
	}
	if !info.Mode().IsRegular() {
		return "", "", 0, errno.New(errno.CodeInvalidArg, "sftp.upload_regular_only", "")
	}
	return localPath, filepath.Base(localPath), info.Size(), nil
}

func ValidateDownloadDestination(localPath string) (string, error) {
	localPath = filepath.Clean(strings.TrimSpace(localPath))
	if !filepath.IsAbs(localPath) || filepath.Base(localPath) == "." {
		return "", errno.New(errno.CodeInvalidArg, "sftp.destination_invalid", "")
	}
	parent, err := os.Stat(filepath.Dir(localPath))
	if err != nil || !parent.IsDir() {
		return "", errno.New(errno.CodeNotFound, "sftp.destination_missing", "")
	}
	return localPath, nil
}

func RemoteDirectoryEntries(directory string, entries []fs.FileInfo) ([]model.RemoteFileEntry, error) {
	result := make([]model.RemoteFileEntry, 0, len(entries))
	for _, entry := range entries {
		value, err := model.NewRemoteFileEntry(directory, entry)
		if err != nil {
			return nil, err
		}
		if stat, ok := entry.Sys().(*pkgsftp.FileStat); ok && stat != nil {
			value.UID = stat.UID
			value.GID = stat.GID
		}
		result = append(result, value)
	}
	sort.SliceStable(result, func(left, right int) bool {
		leftDirectory := result[left].Type == model.RemoteFileTypeDirectory
		rightDirectory := result[right].Type == model.RemoteFileTypeDirectory
		if leftDirectory != rightDirectory {
			return leftDirectory
		}
		leftName := strings.ToLower(result[left].Name)
		rightName := strings.ToLower(result[right].Name)
		if leftName == rightName {
			return result[left].Name < result[right].Name
		}
		return leftName < rightName
	})
	return result, nil
}

func TransferServiceError(err error) error {
	switch {
	case errors.Is(err, ErrSFTPTransferNotFound):
		return errno.New(errno.CodeNotFound, "sftp.not_found", "")
	case errors.Is(err, ErrSFTPTransferNotConfirmable):
		return errno.New(errno.CodeInvalidArg, "sftp.not_confirmable", "")
	case errors.Is(err, ErrSFTPTransferNotRetryable):
		return errno.New(errno.CodeInvalidArg, "sftp.not_retryable", "")
	case errors.Is(err, ErrServiceNotReady),
		errors.Is(err, ErrSFTPTransferManagerClosed),
		errors.Is(err, ErrSFTPTransferExecutorNotReady):
		return ServiceError(ErrServiceNotReady)
	case errors.Is(err, ErrSFTPTransferDestinationExists):
		return errno.New(errno.CodePermissionDenied, "sftp.destination_exists", "")
	case errors.Is(err, ErrSFTPTransferInvalidLocalSource):
		return errno.New(errno.CodeInvalidArg, "sftp.local_path_invalid", "")
	case errors.Is(err, sessionapp.ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, "session.not_found", "")
	case errors.Is(err, sessionapp.ErrSessionNotConnected):
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, fs.ErrExist):
		return errno.New(errno.CodePermissionDenied, "sftp.destination_exists", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, "sftp.not_found", "")
	case errors.Is(err, fs.ErrPermission):
		return errno.New(errno.CodePermissionDenied, "sftp.permission_denied", "")
	default:
		return errno.New(errno.CodeConnFailed, "sftp.failed", "")
	}
}

func ServiceError(err error) error {
	if errors.Is(err, ErrServiceNotReady) {
		return errno.New(errno.CodeSessionClosed, "error.serviceNotReady", "")
	}
	switch {
	case errors.Is(err, sessionapp.ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, "session.not_found", "")
	case errors.Is(err, sessionapp.ErrSessionNotConnected):
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	case errors.Is(err, sessionapp.ErrSessionFileSystemUnsupported):
		return errno.New(errno.CodeInvalidArg, "sftp.unsupported", "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, fs.ErrPermission):

		return errno.New(errno.CodePermissionDenied, "sftp.directory_permission_denied", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, "sftp.remote_directory_not_found", "")
	default:
		return errno.New(errno.CodeConnFailed, "sftp.list_failed", "")
	}
}

func RemotePathError(err error, fallbackKey string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrPermission):
		return errno.New(errno.CodePermissionDenied, "sftp.directory_permission_denied", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
	default:
		return errno.New(errno.CodeConnFailed, fallbackKey, "")
	}
}
