package tunnel

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileSystemGroup struct {
	connection *ssh.Client

	provisioner ConnectionProvisioner

	desiredConnections int
	connections        []*connectionEntry
	dialFailedAt       time.Time

	subsystemWidth int
	systemsPerConn int

	openSystemFn func(context.Context) (*FileSystem, error)

	capabilities serverCapabilities

	expanded atomic.Int64
	degraded atomic.Int64

	expanding     bool
	expansionDone chan struct{}

	mu       sync.Mutex
	idle     []*subsystem
	open     int
	nextID   int
	stopped  bool
	draining bool
	stopErr  error
	cond     *sync.Cond

	stateChanged chan struct{}
}

type connectionEntry struct {
	client *ssh.Client

	closeFn func() error
	inUse   int
}

const sshDialRetryCooldown = 30 * time.Second

type subsystem struct {
	fileSystem *FileSystem
	group      *FileSystemGroup

	connection int
}

func newFileSystemGroup(
	primary *ssh.Client,
	provisioner ConnectionProvisioner,
	subsystemWidth int,
) *FileSystemGroup {
	if subsystemWidth < 1 {
		subsystemWidth = 1
	}
	if subsystemWidth > sftpParallelMaxWidth {
		subsystemWidth = sftpParallelMaxWidth
	}
	group := &FileSystemGroup{
		connection:         primary,
		provisioner:        provisioner,
		desiredConnections: 1,
		subsystemWidth:     subsystemWidth,
		connections:        []*connectionEntry{{client: primary}},
	}
	group.refreshCapacityLocked()
	group.cond = sync.NewCond(&group.mu)
	group.stateChanged = make(chan struct{})
	return group
}

func newFileSystemGroupWithOpener(
	maxOpen int,
	open func(context.Context) (*FileSystem, error),
) *FileSystemGroup {
	if maxOpen < 1 {
		maxOpen = 1
	}
	group := &FileSystemGroup{
		desiredConnections: 1,
		subsystemWidth:     maxOpen,
		connections:        []*connectionEntry{{}},
		openSystemFn:       open,
	}
	group.refreshCapacityLocked()
	group.cond = sync.NewCond(&group.mu)
	group.stateChanged = make(chan struct{})
	return group
}

func (g *FileSystemGroup) notifyStateChangedLocked() {
	if g.stateChanged == nil {
		g.stateChanged = make(chan struct{})
		return
	}
	close(g.stateChanged)
	g.stateChanged = make(chan struct{})
}

func (g *FileSystemGroup) refreshCapacityLocked() {
	connections := len(g.connections)
	if connections < 1 {
		connections = 1
	}
	per := (g.subsystemWidth + connections - 1) / connections
	if per < 1 {
		per = 1
	}
	if per > sftpSubsystemPerConnection {
		per = sftpSubsystemPerConnection
	}
	g.systemsPerConn = per
}

func (g *FileSystemGroup) ConnectionCount() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.connections)
}

func (g *FileSystemGroup) DesiredConnections() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.desiredConnections
}

func (g *FileSystemGroup) ExpansionCounters() (int, int) {
	if g == nil {
		return 0, 0
	}
	return int(g.expanded.Load()), int(g.degraded.Load())
}

func (g *FileSystemGroup) ExpansionBlocked() string {
	return g.expansionBlockedFor(g.DesiredConnections())
}

func (g *FileSystemGroup) expansionBlockedFor(desired int) string {
	if g == nil {
		return "pool_not_ready"
	}
	desired = normalizedConnectionCount(desired)
	g.mu.Lock()
	defer g.mu.Unlock()
	if desired <= 1 {
		return "single_connection_requested"
	}
	if len(g.connections) >= desired {
		return ""
	}
	if g.provisioner == nil {

		return "credential_unavailable"
	}
	if !g.dialFailedAt.IsZero() && time.Since(g.dialFailedAt) < sshDialRetryCooldown {
		return "retry_cooldown"
	}
	return "expansion_pending"
}

func normalizedConnectionCount(count int) int {
	if count > sshMaxConnectionsPerSession {
		return sshMaxConnectionsPerSession
	}
	if count < 1 {
		return 1
	}
	return count
}

func (g *FileSystemGroup) effectiveCapacity() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	capacity := len(g.connections) * g.systemsPerConn
	if capacity < 1 {
		capacity = 1
	}
	return capacity
}

func (g *FileSystemGroup) setConnections(count int) {
	if g == nil {
		return
	}
	count = normalizedConnectionCount(count)
	g.mu.Lock()
	if count > g.desiredConnections {
		g.desiredConnections = count
	}
	if g.cond != nil {
		g.cond.Broadcast()
	}
	g.notifyStateChangedLocked()
	g.mu.Unlock()
}

func (g *FileSystemGroup) waitForConnection(ctx context.Context, target int) (bool, error) {
	if g == nil {
		return false, ErrFileSystemNotReady
	}
	if ctx == nil {
		return false, errors.New("SFTP connection wait context is nil")
	}
	if target < 1 {
		target = 1
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		g.mu.Lock()
		if len(g.connections) >= target {
			g.mu.Unlock()
			return true, nil
		}
		coolingDown := !g.dialFailedAt.IsZero() && time.Since(g.dialFailedAt) < sshDialRetryCooldown
		if g.stopped || g.draining || g.provisioner == nil ||
			(!g.expanding && coolingDown) {
			g.mu.Unlock()
			return false, nil
		}
		changed := g.stateChanged
		g.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func (g *FileSystemGroup) ensureConnections(ctx context.Context, target int) {
	if g == nil || ctx == nil {
		return
	}
	target = normalizedConnectionCount(target)
	g.setConnections(target)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		g.mu.Lock()
		if g.stopped || g.draining || g.provisioner == nil || len(g.connections) >= target {
			g.mu.Unlock()
			return
		}
		if !g.dialFailedAt.IsZero() && time.Since(g.dialFailedAt) < sshDialRetryCooldown {
			g.mu.Unlock()
			return
		}
		if g.expanding {
			done := g.expansionDone
			g.mu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return
			}
		}
		g.expanding = true
		done := make(chan struct{})
		g.expansionDone = done
		provisioner := g.provisioner
		desired := target
		g.mu.Unlock()

		auxiliary, dialErr := provisioner(ctx)
		if dialErr == nil && auxiliary.Client == nil {
			dialErr = errors.New("SFTP auxiliary connection is empty")
		}

		var closeAuxiliary func() error
		var added bool
		var connections, systemsPerConnection int
		var degradedErr error
		g.mu.Lock()
		if dialErr == nil && auxiliary.Client != nil && ctx.Err() == nil && !g.stopped && !g.draining && len(g.connections) < desired {
			g.connections = append(g.connections, &connectionEntry{client: auxiliary.Client, closeFn: auxiliary.Close})
			g.refreshCapacityLocked()
			g.dialFailedAt = time.Time{}
			g.expanded.Add(1)
			connections = len(g.connections)
			systemsPerConnection = g.systemsPerConn
			added = true
		} else {
			closeAuxiliary = auxiliary.Close
			if dialErr != nil && ctx.Err() == nil && !g.stopped && !g.draining {
				g.dialFailedAt = time.Now()
				g.degraded.Add(1)
				degradedErr = dialErr
			}
		}
		g.expanding = false
		g.expansionDone = nil
		close(done)
		if g.cond != nil {
			g.cond.Broadcast()
		}
		g.notifyStateChangedLocked()
		g.mu.Unlock()

		if closeAuxiliary != nil && auxiliary.Client != nil {
			_ = closeAuxiliary()
		} else if auxiliary.Client != nil && !added {
			_ = auxiliary.Client.Close()
		}
		if added {
			slog.Info("sftp auxiliary connection established",
				"event", "sftp.expand",
				"connections", connections,
				"desired", desired,
				"systems_per_connection", systemsPerConnection)
			continue
		}
		if degradedErr != nil {

			slog.Info("sftp auxiliary connection unavailable, keeping current connections",
				"event", "sftp.expand.degraded",
				"connections", g.ConnectionCount(),
				"desired", desired,
				"error", degradedErr)
		}
		return
	}
}

func (g *FileSystemGroup) acquire(ctx context.Context) (*PooledFileSystem, error) {
	return g.acquireForConnections(ctx, 1)
}

func (g *FileSystemGroup) acquireForConnections(
	ctx context.Context,
	desired int,
) (*PooledFileSystem, error) {
	desired = normalizedConnectionCount(desired)
	g.setConnections(desired)
	borrowed, err := g.acquireSystem(ctx)
	if err != nil {
		return nil, err
	}
	return &PooledFileSystem{borrowed: borrowed, desiredConnections: desired}, nil
}

func (g *FileSystemGroup) acquireSystem(ctx context.Context) (*subsystem, error) {
	if g == nil || (g.connection == nil && g.openSystemFn == nil && len(g.connections) == 0) {
		return nil, ErrFileSystemNotReady
	}
	if ctx == nil {
		return nil, errors.New("SFTP subsystem pool context is nil")
	}
	stopCancellationWake := context.AfterFunc(ctx, func() {
		g.mu.Lock()
		if g.cond != nil {
			g.cond.Broadcast()
		}
		g.mu.Unlock()
	})
	defer stopCancellationWake()
	expansionAttempted := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		if g.stopped {
			err := g.stopErr
			g.mu.Unlock()
			if err == nil {
				err = errFileSystemPoolClosed
			}
			return nil, err
		}

		if idleIndex := g.lowestLoadIdleIndexLocked(); idleIndex >= 0 {
			last := len(g.idle) - 1
			system := g.idle[idleIndex]
			g.idle[idleIndex] = g.idle[last]
			g.idle = g.idle[:last]
			g.connections[system.connection].inUse++
			g.mu.Unlock()
			return system, nil
		}
		if g.draining {

			g.mu.Unlock()
			return nil, errFileSystemPoolDraining
		}

		if index := g.lowestLoadIndexLocked(); index >= 0 {
			g.open++
			entry := g.connections[index]
			entry.inUse++
			client := entry.client
			g.mu.Unlock()
			system, err := g.openSystemOn(ctx, client, index)
			if err != nil {
				g.mu.Lock()
				g.open--
				entry.inUse--
				g.cond.Broadcast()
				g.mu.Unlock()
				return nil, err
			}
			g.mu.Lock()
			if g.stopped || g.draining {
				g.open--
				entry.inUse--
				err := g.stopErr
				if err == nil {
					err = errFileSystemPoolDraining
				}
				g.cond.Broadcast()
				g.mu.Unlock()
				_ = system.fileSystem.Close()
				return nil, err
			}
			g.nextID++
			g.mu.Unlock()
			return system, nil
		}

		if len(g.connections) < g.desiredConnections {
			if !expansionAttempted {
				expansionAttempted = true
				target := g.desiredConnections
				g.mu.Unlock()
				g.ensureConnections(ctx, target)
				continue
			}
		}

		if err := ctx.Err(); err != nil {
			g.mu.Unlock()
			return nil, err
		}
		g.cond.Wait()
		g.mu.Unlock()
		expansionAttempted = false
	}
}

func (g *FileSystemGroup) acquireSystemOnConnection(
	ctx context.Context,
	connection int,
) (*subsystem, error) {
	if g == nil {
		return nil, ErrFileSystemNotReady
	}
	if ctx == nil {
		return nil, errors.New("SFTP subsystem pool context is nil")
	}
	stopCancellationWake := context.AfterFunc(ctx, func() {
		g.mu.Lock()
		if g.cond != nil {
			g.cond.Broadcast()
		}
		g.mu.Unlock()
	})
	defer stopCancellationWake()

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		if g.stopped {
			err := g.stopErr
			g.mu.Unlock()
			if err == nil {
				err = errFileSystemPoolClosed
			}
			return nil, err
		}
		if connection < 0 || connection >= len(g.connections) {
			g.mu.Unlock()
			return nil, ErrFileSystemNotReady
		}
		for idleIndex, system := range g.idle {
			if system == nil || system.connection != connection {
				continue
			}
			last := len(g.idle) - 1
			g.idle[idleIndex] = g.idle[last]
			g.idle = g.idle[:last]
			g.connections[connection].inUse++
			g.mu.Unlock()
			return system, nil
		}
		if g.draining {
			g.mu.Unlock()
			return nil, errFileSystemPoolDraining
		}
		entry := g.connections[connection]
		if entry.inUse < g.systemsPerConn {
			g.open++
			entry.inUse++
			client := entry.client
			g.mu.Unlock()
			system, err := g.openSystemOn(ctx, client, connection)
			if err != nil {
				g.mu.Lock()
				g.open--
				entry.inUse--
				g.cond.Broadcast()
				g.mu.Unlock()
				return nil, err
			}
			g.mu.Lock()
			if g.stopped || g.draining {
				g.open--
				entry.inUse--
				err := g.stopErr
				if err == nil {
					err = errFileSystemPoolDraining
				}
				g.cond.Broadcast()
				g.mu.Unlock()
				_ = system.fileSystem.Close()
				return nil, err
			}
			g.nextID++
			g.mu.Unlock()
			return system, nil
		}
		if err := ctx.Err(); err != nil {
			g.mu.Unlock()
			return nil, err
		}
		g.cond.Wait()
		g.mu.Unlock()
	}
}

func (g *FileSystemGroup) lowestLoadIdleIndexLocked() int {
	best := -1
	for index, system := range g.idle {
		if system == nil || system.connection < 0 || system.connection >= len(g.connections) {
			continue
		}
		if best < 0 ||
			g.connections[system.connection].inUse < g.connections[g.idle[best].connection].inUse {
			best = index
		}
	}
	return best
}

func (g *FileSystemGroup) lowestLoadIndexLocked() int {
	best := -1
	for index, entry := range g.connections {
		if entry.inUse >= g.systemsPerConn {
			continue
		}
		if best < 0 || entry.inUse < g.connections[best].inUse {
			best = index
		}
	}
	return best
}

func (g *FileSystemGroup) openSystemOn(ctx context.Context, client *ssh.Client, index int) (*subsystem, error) {
	open := g.openSystemFn
	if open == nil {
		open = func(ctx context.Context) (*FileSystem, error) {
			return openSFTPSubsystem(ctx, client)
		}
	}
	system, err := open(ctx)
	if err != nil {
		return nil, err
	}
	system.group = g
	system.capabilities = &g.capabilities
	return &subsystem{fileSystem: system, group: g, connection: index}, nil
}

func (g *FileSystemGroup) release(borrowed *subsystem, broken bool) {
	if g == nil || borrowed == nil || borrowed.fileSystem == nil {
		return
	}
	g.mu.Lock()
	if borrowed.connection >= 0 && borrowed.connection < len(g.connections) {
		if entry := g.connections[borrowed.connection]; entry.inUse > 0 {
			entry.inUse--
		}
	}
	if broken || g.stopped || g.draining {
		g.open--
		g.cond.Broadcast()
		g.mu.Unlock()
		_ = borrowed.fileSystem.Close()
		return
	}
	g.idle = append(g.idle, borrowed)
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *FileSystemGroup) abort(err error) {
	if g == nil || err == nil {
		return
	}
	g.mu.Lock()
	g.stopped = true
	g.stopErr = err
	g.cond.Broadcast()
	g.notifyStateChangedLocked()
	g.mu.Unlock()
}

func (g *FileSystemGroup) shutdown() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.draining = true
	idle := g.idle
	g.idle = nil
	g.open -= len(idle)

	auxiliary := make([]func() error, 0, len(g.connections))
	for index, entry := range g.connections {
		if index > 0 && entry.closeFn != nil {
			auxiliary = append(auxiliary, entry.closeFn)
		}
	}

	g.cond.Broadcast()
	g.notifyStateChangedLocked()
	g.mu.Unlock()
	for _, borrowed := range idle {
		_ = borrowed.fileSystem.Close()
	}
	for _, closeFn := range auxiliary {
		_ = closeFn()
	}
}

var errFileSystemPoolClosed = errors.New("SFTP subsystem pool is closed")

var errFileSystemPoolDraining = errors.New("SFTP subsystem pool is draining")

type PooledFileSystem struct {
	borrowed           *subsystem
	desiredConnections int
	releaseOnce        sync.Once
	parallelBroken     atomic.Bool
}

func (p *PooledFileSystem) recordOperationResult(ctx context.Context, err error) error {
	if p == nil {
		return err
	}
	if (ctx != nil && ctx.Err() != nil) || isBrokenSubsystemError(err) {
		p.parallelBroken.Store(true)
	}
	return err
}

func (p *PooledFileSystem) Close() error {
	if p == nil {
		return nil
	}
	p.releaseOnce.Do(func() {
		if p.borrowed != nil {
			p.borrowed.group.release(p.borrowed, p.parallelBroken.Load())
		}
	})
	return nil
}

func (p *PooledFileSystem) markParallelSeedBroken(broken bool) {
	if p != nil && broken {
		p.parallelBroken.Store(true)
	}
}

func (p *PooledFileSystem) WorkingDirectory(ctx context.Context) (string, error) {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return "", ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.WorkingDirectory(ctx)
}

func (p *PooledFileSystem) ReadDirContext(ctx context.Context, directory string) ([]fs.FileInfo, error) {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return nil, ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.ReadDirContext(ctx, directory)
}

func (p *PooledFileSystem) StatContext(ctx context.Context, remotePath string) (fs.FileInfo, error) {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return nil, ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.StatContext(ctx, remotePath)
}

func (p *PooledFileSystem) UploadFileContext(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.UploadFileContext(ctx, localPath, remotePath, expectedSize, overwrite, progress)
}

func (p *PooledFileSystem) DownloadFileContext(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.DownloadFileContext(ctx, remotePath, localPath, expectedSize, overwrite, progress)
}

func (p *PooledFileSystem) UploadFileParallel(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	preserveDestination bool,
	progress func(int64) error,
) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.uploadFileParallelWithSeed(
		ctx, localPath, remotePath, expectedSize, overwrite, preserveDestination, progress,
		p.borrowed, p.markParallelSeedBroken, p.desiredConnections,
	)
}

func (p *PooledFileSystem) UploadFileParallelResumable(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
	resumeKey string,
) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.UploadFileParallelResumable(
		ctx, localPath, remotePath, expectedSize, overwrite, progress, resumeKey,
	)
}

func (p *PooledFileSystem) DropResumeState(resumeKey string) string {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ""
	}
	return p.borrowed.fileSystem.DropResumeState(resumeKey)
}

func (p *PooledFileSystem) DownloadFileParallel(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.downloadFileParallelWithSeed(
		ctx, remotePath, localPath, expectedSize, overwrite, progress,
		p.borrowed, p.markParallelSeedBroken, p.desiredConnections,
	)
}

func (p *PooledFileSystem) TransferConnectionStatus() (int, int, string) {
	if p == nil || p.borrowed == nil || p.borrowed.group == nil {
		return 0, 0, "pool_not_ready"
	}
	group := p.borrowed.group
	desired := normalizedConnectionCount(p.desiredConnections)
	active := group.ConnectionCount()
	if active > desired {

		active = desired
	}
	return active, desired, group.expansionBlockedFor(desired)
}

func (p *PooledFileSystem) TransferExpansionCounters() (int, int) {
	if p == nil || p.borrowed == nil || p.borrowed.group == nil {
		return 0, 0
	}
	return p.borrowed.group.ExpansionCounters()
}

func (p *PooledFileSystem) TemporaryUploadPath(err error) string {
	var uploadErr *parallelUploadError
	if errors.As(err, &uploadErr) {
		return uploadErr.TemporaryPath()
	}
	return ""
}

func (p *PooledFileSystem) OpenFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return nil, ErrFileSystemNotReady
	}
	file, err := p.borrowed.fileSystem.OpenFile(ctx, path)
	if err != nil {
		return nil, p.recordOperationResult(ctx, err)
	}

	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		p.parallelBroken.Store(true)
		_ = p.borrowed.fileSystem.Close()
		close(done)
	})
	return &pooledReadCloser{ReadCloser: file, stop: stop, done: done}, nil
}

type pooledReadCloser struct {
	io.ReadCloser
	stop      func() bool
	done      <-chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func (r *pooledReadCloser) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.closeErr = r.ReadCloser.Close()
		if !r.stop() {
			<-r.done
		}
	})
	return r.closeErr
}

func (p *PooledFileSystem) ChownEntryContext(ctx context.Context, remotePath string, uid, gid uint32) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.ChownEntryContext(ctx, remotePath, uid, gid))
}

func (p *PooledFileSystem) ChmodEntryContext(ctx context.Context, remotePath string, mode os.FileMode) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.ChmodEntryContext(ctx, remotePath, mode))
}

func (p *PooledFileSystem) CreateDirectoryContext(ctx context.Context, remotePath string) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.CreateDirectoryContext(ctx, remotePath))
}

func (p *PooledFileSystem) CreateFileContext(ctx context.Context, remotePath string) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.CreateFileContext(ctx, remotePath))
}

func (p *PooledFileSystem) DeleteEntryContext(ctx context.Context, remotePath string) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.DeleteEntryContext(ctx, remotePath))
}

func (p *PooledFileSystem) RenameEntryContext(ctx context.Context, oldPath, newPath string) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.recordOperationResult(ctx, p.borrowed.fileSystem.RenameEntryContext(ctx, oldPath, newPath))
}

func (p *PooledFileSystem) CreateTransferFile(ctx context.Context, finalPath string) (string, io.WriteCloser, error) {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return "", nil, ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.CreateTransferFile(ctx, finalPath)
}

func (p *PooledFileSystem) CommitTransferFile(ctx context.Context, temporaryPath string, finalPath string, overwrite bool) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.CommitTransferFile(ctx, temporaryPath, finalPath, overwrite)
}

func (p *PooledFileSystem) RemoveTransferFile(ctx context.Context, remotePath string) error {
	if p == nil || p.borrowed == nil || p.borrowed.fileSystem == nil {
		return ErrFileSystemNotReady
	}
	return p.borrowed.fileSystem.RemoveTransferFile(ctx, remotePath)
}

func openSFTPSubsystem(ctx context.Context, connection *ssh.Client) (*FileSystem, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, ErrFileSystemNotReady
	}

	session, err := connection.NewSession()
	if err != nil {
		return nil, fileSystemOpenError(ctx, "create SFTP session", err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		_ = session.Close()
		return nil, contextErr
	}
	cancellationDone := make(chan struct{})
	stopCancellation := context.AfterFunc(ctx, func() {
		defer close(cancellationDone)
		_ = session.Close()
	})
	var stopCancellationOnce sync.Once
	stopAndWaitCancellation := func() {
		stopCancellationOnce.Do(func() {
			if !stopCancellation() {
				<-cancellationDone
			}
		})
	}
	defer stopAndWaitCancellation()
	if contextErr := ctx.Err(); contextErr != nil {
		_ = session.Close()
		return nil, contextErr
	}
	opened := false
	defer func() {
		if !opened {
			_ = session.Close()
		}
	}()
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fileSystemOpenError(ctx, "open SFTP input", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fileSystemOpenError(ctx, "open SFTP output", err)
	}
	session.Stderr = io.Discard
	if err := session.RequestSubsystem("sftp"); err != nil {
		return nil, fileSystemOpenError(ctx, "request SFTP subsystem", err)
	}
	client, err := newSFTPClient(stdout, stdin)
	if contextErr := ctx.Err(); contextErr != nil {
		if client != nil {
			_ = client.Close()
		}
		return nil, contextErr
	}
	if err != nil {
		return nil, fileSystemOpenError(ctx, "negotiate SFTP subsystem", err)
	}
	stopAndWaitCancellation()
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, ctx.Err()
	}
	opened = true
	return &FileSystem{client: client, session: session}, nil
}

func (f *FileSystem) sftpClient() *sftp.Client {
	if f == nil {
		return nil
	}
	return f.client
}
