package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
)

type parallelTransferError struct {
	err error
}

func (e *parallelTransferError) Error() string {
	return fmt.Sprintf("parallel SFTP transfer: %v", e.err)
}

func (e *parallelTransferError) Unwrap() error { return e.err }

type chunkSource func(system *FileSystem, buffer []byte, offset int64, report func(int64) error) error

type chunkSink func(system *FileSystem, buffer []byte, offset int64, report func(int64) error) error

type parallelChunkWorker struct {
	source chunkSource
	sink   chunkSink
	close  func() error
}

type parallelChunkWorkerFactory func(system *FileSystem) (parallelChunkWorker, error)

const (
	parallelProgressReportBytes = 256 * 1024

	parallelProgressReportMinEvery = 25 * time.Millisecond
	parallelProgressReportEvery    = 100 * time.Millisecond
)

type parallelProgressReporter struct {
	callback func(int64) error
	total    int64

	mu            sync.Mutex
	completed     int64
	pending       int64
	lastReportAt  time.Time
	callbackError error
}

func newParallelProgressReporter(progress func(int64) error, total int64) *parallelProgressReporter {
	if progress == nil {
		return nil
	}
	return &parallelProgressReporter{
		callback:     progress,
		total:        total,
		lastReportAt: time.Now(),
	}
}

func (r *parallelProgressReporter) add(delta int64) error {
	if r == nil || delta <= 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.callbackError != nil {
		return r.callbackError
	}
	remaining := r.total - r.completed
	if remaining <= 0 {
		return nil
	}
	if delta > remaining {
		delta = remaining
	}
	r.completed += delta
	r.pending += delta
	if r.completed < r.total {
		elapsed := time.Since(r.lastReportAt)
		if elapsed < parallelProgressReportEvery &&
			(r.pending < parallelProgressReportBytes || elapsed < parallelProgressReportMinEvery) {
			return nil
		}
	}
	return r.flushLocked()
}

func (r *parallelProgressReporter) finish() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.callbackError != nil {
		return r.callbackError
	}
	if r.completed != r.total {
		return fmt.Errorf("parallel transfer progress = %d, want %d", r.completed, r.total)
	}
	if r.pending == 0 {
		return nil
	}
	return r.flushLocked()
}

func (r *parallelProgressReporter) flushLocked() error {
	if r.pending == 0 {
		return nil
	}
	r.pending = 0
	r.lastReportAt = time.Now()
	if err := r.callback(r.completed); err != nil {
		r.callbackError = err
		return err
	}
	return nil
}

func runParallelChunks(
	ctx context.Context,
	pool *FileSystemGroup,
	plan sftpParallelPlan,
	size int64,
	progress func(transferred int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
	resume *SFTPResumeManifest,
	workerFactory parallelChunkWorkerFactory,
) (resultErr error) {
	if pool == nil {
		return ErrFileSystemNotReady
	}
	if ctx == nil {
		return errors.New("SFTP parallel transfer context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if seed == nil || releaseSeed == nil || seed.group != pool || seed.fileSystem == nil {
		seed = nil
		releaseSeed = nil
	}
	if workerFactory == nil {
		return errors.New("SFTP parallel worker factory is nil")
	}

	chunkCount := int((size + plan.chunkSize - 1) / plan.chunkSize)
	width := plan.width
	if width > chunkCount {
		width = chunkCount
	}

	desiredConnections = normalizedConnectionCount(desiredConnections)
	if width > desiredConnections {
		width = desiredConnections
	}
	if width < 1 {
		width = 1
	}
	startedAt := time.Now()
	var workersStarted atomic.Int64
	var bytesByConnection [sshMaxConnectionsPerSession]atomic.Int64
	defer func() {
		connectionCount := pool.ConnectionCount()
		if connectionCount > len(bytesByConnection) {
			connectionCount = len(bytesByConnection)
		}
		connectionBytes := make([]int64, connectionCount)
		for index := range connectionBytes {
			connectionBytes[index] = bytesByConnection[index].Load()
		}
		slog.Info("sftp parallel workers finished",
			"event", "sftp.parallel.done",
			"desired_connections", desiredConnections,
			"workers_started", workersStarted.Load(),
			"bytes_by_connection", connectionBytes,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"success", resultErr == nil)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	acquireCtx, stopAcquiring := context.WithCancel(ctx)
	defer stopAcquiring()

	expansionCtx, stopExpansion := context.WithCancel(ctx)
	expansionDone := make(chan struct{})
	expansionStarted := make(chan struct{})
	go func() {
		defer close(expansionDone)
		close(expansionStarted)
		pool.ensureConnections(expansionCtx, desiredConnections)
	}()
	defer func() {
		stopExpansion()
		<-expansionDone
	}()

	work := make(chan int)
	var firstErr atomic.Pointer[parallelTransferError]

	reporter := newParallelProgressReporter(progress, size)

	if resume != nil {
		if done := resume.CompletedBytes(); done > 0 && done <= size {
			reporter.completed = done
		}
	}
	skipChunks := resume.Skip()

	buffers := make([][]byte, width)
	for index := range buffers {
		buffers[index] = make([]byte, plan.chunkSize)
	}

	var workers sync.WaitGroup
	for index := 0; index < width; index++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			borrowed := (*subsystem)(nil)
			seedForWorker := false
			if worker == 0 && seed != nil {
				borrowed = seed
				seedForWorker = true
			} else {

				targetConnection := -1
				if worker > 0 {
					select {
					case <-expansionStarted:
					case <-acquireCtx.Done():
						return
					}
					targetConnection = worker
					if seed != nil && seed.connection >= 0 {

						targetConnection = worker - 1
						if targetConnection >= seed.connection {
							targetConnection++
						}
					}
					ready, err := pool.waitForConnection(acquireCtx, targetConnection+1)
					if err != nil || !ready {
						return
					}
				}
				var err error
				if targetConnection >= 0 {
					borrowed, err = pool.acquireSystemOnConnection(acquireCtx, targetConnection)
				} else {
					borrowed, err = pool.acquireSystem(acquireCtx)
				}
				if err != nil {

					if ctx.Err() != nil {
						recordFirstError(&firstErr, cancel, err)
					}
					return
				}
			}
			broken := false
			stopCancellation := watchSubsystemCancellation(ctx, borrowed)
			workerState, err := workerFactory(borrowed.fileSystem)
			stopCancellation()
			if err != nil {
				broken = isBrokenSubsystemError(err) || ctx.Err() != nil
				recordFirstError(&firstErr, cancel, err)
				if seedForWorker {
					releaseSeed(broken)
				} else {
					borrowed.group.release(borrowed, broken)
				}
				return
			}

			stopCancellation = watchSubsystemCancellation(ctx, borrowed)
			if workerState.source == nil || workerState.sink == nil {
				err = errors.New("SFTP parallel worker callbacks are incomplete")
				if workerState.close != nil {
					if closeErr := workerState.close(); closeErr != nil {
						err = fmt.Errorf("%w: close resource: %v", err, closeErr)
					}
				}
				stopCancellation()
				recordFirstError(&firstErr, cancel, err)
				if seedForWorker {
					releaseSeed(true)
				} else {
					borrowed.group.release(borrowed, true)
				}
				return
			}
			workersStarted.Add(1)
			buffer := buffers[worker]
			for chunk := range work {
				if firstErr.Load() != nil {

					continue
				}
				offset := int64(chunk) * plan.chunkSize
				length := plan.chunkSize
				if remaining := size - offset; remaining < length {
					length = remaining
				}
				window := buffer[:length]

				system := borrowed.fileSystem
				if err := workerState.source(system, window, offset, reporter.add); err != nil {
					broken = broken || isBrokenSubsystemError(err) || ctx.Err() != nil
					recordFirstError(&firstErr, cancel, err)
					continue
				}
				chunkErr := workerState.sink(system, window, offset, reporter.add)
				broken = broken || isBrokenSubsystemError(chunkErr) || ctx.Err() != nil
				if chunkErr != nil {
					recordFirstError(&firstErr, cancel, chunkErr)
					continue
				}
				if borrowed.connection >= 0 && borrowed.connection < len(bytesByConnection) {
					bytesByConnection[borrowed.connection].Add(int64(length))
				}

				if resume != nil {
					resume.MarkChunkDone(int64(chunk), int64(length), resumeChunkDigest(window))
				}
			}
			if workerState.close != nil {
				closeErr := workerState.close()
				if closeErr != nil && ctx.Err() == nil {
					broken = true
					recordFirstError(&firstErr, cancel, fmt.Errorf("close parallel worker resource: %w", closeErr))
				}
			}

			stopCancellation()
			broken = broken || ctx.Err() != nil
			if seedForWorker {
				releaseSeed(broken)
				return
			}
			borrowed.group.release(borrowed, broken)
		}(index)
	}

	for chunk := 0; chunk < chunkCount; chunk++ {
		if _, done := skipChunks[int64(chunk)]; done {

			continue
		}
		select {
		case work <- chunk:
		case <-ctx.Done():

			close(work)
			stopAcquiring()
			workers.Wait()
			if failure := firstErr.Load(); failure != nil {
				return failure
			}
			return ctx.Err()
		}
	}
	close(work)

	stopAcquiring()
	workers.Wait()
	if failure := firstErr.Load(); failure != nil {
		return failure
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := reporter.finish(); err != nil {
		return &parallelTransferError{err: err}
	}
	return nil
}

func watchSubsystemCancellation(ctx context.Context, borrowed *subsystem) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		_ = borrowed.fileSystem.Close()
	})
	return func() {
		if stop() {
			return
		}
		<-done
	}
}

func recordFirstError(slot *atomic.Pointer[parallelTransferError], cancel context.CancelFunc, err error) {
	if err == nil {
		return
	}
	if slot.CompareAndSwap(nil, &parallelTransferError{err: err}) {
		cancel()
	}
}

func isBrokenSubsystemError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return errors.Is(err, sftp.ErrSSHFxConnectionLost)
}
