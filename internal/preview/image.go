package preview

import (
	"context"
	"encoding/base64"
	"io"
	"io/fs"
	"path"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	maxRasterImageBytes = 16 * 1024 * 1024

	maxSVGImageBytes = 2 * 1024 * 1024

	svgExtension = ".svg"
)

var imageMimeByExtension = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
	".cur":  "image/x-icon",
	".svg":  "image/svg+xml",
	".avif": "image/avif",
}

func imageTypeFor(remotePath string) (mimeType string, limit int64, supported bool) {
	extension := strings.ToLower(path.Ext(remotePath))
	mimeType, supported = imageMimeByExtension[extension]
	if !supported {
		return "", 0, false
	}
	if extension == svgExtension {
		return mimeType, maxSVGImageBytes, true
	}
	return mimeType, maxRasterImageBytes, true
}

type remoteFileAccess interface {
	StatContext(ctx context.Context, remotePath string) (fs.FileInfo, error)
	OpenFile(ctx context.Context, remotePath string) (io.ReadCloser, error)
}

func normalizeRemoteFile(remotePath string) (string, error) {
	normalized, err := model.NormalizeRemotePath(strings.TrimSpace(remotePath))
	if err != nil || normalized == "/" {
		return "", errno.New(errno.CodeInvalidArg, messageKeyPathInvalid, "")
	}
	return normalized, nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
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
