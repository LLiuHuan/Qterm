package tunnel

import (
	"io"
	"time"

	"github.com/pkg/sftp"
)

const (
	sftpInteropSafePacketBytes = 32 * 1024

	sshChannelWindowBytes = 64 * sftpInteropSafePacketBytes

	sftpMaxConcurrentRequestsPerFile = sshChannelWindowBytes / sftpInteropSafePacketBytes

	sftpParallelTransferThresholdBytes = 4 * 1024 * 1024

	sftpParallelChunkBytes = int64(sshChannelWindowBytes)

	sftpParallelMaxWidth = 8

	sftpSubsystemPerConnection = 4

	sshMaxConnectionsPerSession = 4

	sshConnectionTier32MiB  = 32 * 1024 * 1024
	sshConnectionTier256MiB = 256 * 1024 * 1024
	sshConnectionTier2GiB   = 2 * 1024 * 1024 * 1024

	sshHighLatencyThreshold = 80 * time.Millisecond

	sshHighLatencyExtraConnections = 1
)

const ParallelConnectionLimit = sshMaxConnectionsPerSession

func ParallelConnectionsFor(size int64, latency time.Duration, limit int) int {
	return planSSHConnections(size, latency, limit).connections
}

type sshConnectionPlan struct {
	connections int
	byFileSize  int
	byLatency   bool
}

func planSSHConnections(size int64, latency time.Duration, limit int) sshConnectionPlan {
	if limit > sshMaxConnectionsPerSession {
		limit = sshMaxConnectionsPerSession
	}
	if limit < 1 {
		limit = 1
	}
	if limit == 1 || size <= 0 {

		return sshConnectionPlan{connections: 1}
	}

	bySize := 1
	switch {
	case size >= sshConnectionTier2GiB:
		bySize = 4
	case size >= sshConnectionTier256MiB:
		bySize = 3
	case size >= sshConnectionTier32MiB:
		bySize = 2
	}
	if bySize > limit {
		bySize = limit
	}
	if bySize < 1 {
		bySize = 1
	}

	plan := sshConnectionPlan{connections: bySize, byFileSize: bySize}

	if latency >= sshHighLatencyThreshold && bySize+sshHighLatencyExtraConnections <= limit {
		plan.connections = bySize + sshHighLatencyExtraConnections
		plan.byLatency = true
	}
	return plan
}

func sftpClientOptions() []sftp.ClientOption {
	return []sftp.ClientOption{

		sftp.MaxPacketChecked(sftpInteropSafePacketBytes),

		sftp.MaxConcurrentRequestsPerFile(sftpMaxConcurrentRequestsPerFile),

		sftp.UseConcurrentWrites(true),
	}
}

func newSFTPClient(rd io.Reader, wr io.WriteCloser) (*sftp.Client, error) {
	return sftp.NewClientPipe(rd, wr, sftpClientOptions()...)
}

type sftpParallelPlan struct {
	width     int
	chunkSize int64
}

func planSFTPParallelTransfer(size int64) (sftpParallelPlan, bool) {
	if size < sftpParallelTransferThresholdBytes {
		return sftpParallelPlan{}, false
	}
	chunkSize := sftpParallelChunkBytes
	width := int((size + chunkSize - 1) / chunkSize)
	if width > sftpParallelMaxWidth {
		width = sftpParallelMaxWidth
	}
	if width < 2 {

		return sftpParallelPlan{}, false
	}
	return sftpParallelPlan{width: width, chunkSize: chunkSize}, true
}
