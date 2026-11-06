package remoteedit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"time"
	"unicode/utf8"

	"changeme/internal/errno"
)

const (
	maxRemoteTextBytes = 8 * 1024 * 1024

	binaryControlRatio = 0.30
)

type remoteFileAccess interface {
	StatContext(ctx context.Context, remotePath string) (fs.FileInfo, error)
	OpenFile(ctx context.Context, remotePath string) (io.ReadCloser, error)
}

type remoteAtomicWriter interface {
	CreateTransferFile(ctx context.Context, finalPath string) (string, io.WriteCloser, error)
	CommitTransferFile(ctx context.Context, temporaryPath string, finalPath string, overwrite bool) error
	RemoveTransferFile(ctx context.Context, remotePath string) error
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashFile(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func decodeBase64(value string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "remote.edit_content_invalid", "")
	}
	return data, nil
}

func looksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	controls := 0
	for _, b := range data {
		switch b {
		case 0x00:
			return true
		case '\t', '\n', '\v', '\f', '\r':

		default:
			if b < 0x20 || b == 0x7f {
				controls++
			}
		}
	}
	if float64(controls)/float64(len(data)) > binaryControlRatio {
		return true
	}
	return !utf8.Valid(data)
}

func readUpTo(reader io.Reader, limit int64) (data []byte, truncated bool, err error) {
	if limit < 0 {
		limit = 0
	}
	data, err = io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

type remoteObservation struct {
	Exists  bool
	Size    int64
	ModTime time.Time
	Hash    string
}

func observeRemote(
	ctx context.Context,
	fileSystem remoteFileAccess,
	remotePath string,
	limit int64,
) (remoteObservation, bool, error) {
	info, err := fileSystem.StatContext(ctx, remotePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return remoteObservation{Exists: false}, false, nil
		}
		return remoteObservation{}, false, err
	}
	if !info.Mode().IsRegular() {
		return remoteObservation{}, false, errno.New(errno.CodeInvalidArg, "sftp.download_regular_only", "")
	}
	if info.Size() > limit {
		return remoteObservation{Exists: true, Size: info.Size(), ModTime: info.ModTime().UTC()}, true, nil
	}
	reader, err := fileSystem.OpenFile(ctx, remotePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {

			return remoteObservation{Exists: false}, false, nil
		}
		return remoteObservation{}, false, err
	}
	defer reader.Close()
	data, truncated, err := readUpTo(reader, limit)
	if err != nil {
		return remoteObservation{}, false, err
	}
	return remoteObservation{
		Exists:  true,
		Size:    info.Size(),
		ModTime: info.ModTime().UTC(),
		Hash:    hashBytes(data),
	}, truncated, nil
}

func copyLocalFileToWriter(ctx context.Context, localPath string, writer io.Writer) (int64, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	buffer := make([]byte, 64*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			count, writeErr := writer.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}
