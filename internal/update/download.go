package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	progressInterval = 100 * time.Millisecond

	maxChecksumBytes = 1 << 20
)

func (s *Service) downloadAsset(ctx context.Context, release *Release) (string, error) {
	dir := filepath.Join(s.tempRoot, "qterm-update", sanitizeSegment(release.Tag))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", downloadFailed("mkdir")
	}
	target := filepath.Join(dir, release.Asset.Name)
	partial := target + ".part"

	_ = os.Remove(partial)

	sums, err := s.fetchChecksums(ctx)
	if err != nil {
		return "", err
	}
	expected, found := checksumFor(sums, release.Asset.Name)
	if !found {
		return "", checksumMissing()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.Asset.URL, nil)
	if err != nil {
		return "", downloadFailed("request")
	}
	request.Header.Set("User-Agent", "Qterm-Updater")
	response, err := s.downloadClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", downloadFailed("network")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", downloadFailed("HTTP " + strconv.Itoa(response.StatusCode))
	}

	file, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", downloadFailed("open")
	}
	hasher := sha256.New()
	writer := &progressWriter{
		service: s,
		release: release,
		total:   response.ContentLength,
		writer:  io.MultiWriter(file, hasher),
	}

	if writer.total <= 0 {
		writer.total = release.Asset.Size
	}
	written, copyErr := io.Copy(writer, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(partial)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", downloadFailed("copy")
	}
	if closeErr != nil {
		_ = os.Remove(partial)
		return "", downloadFailed("close")
	}
	if writer.total > 0 && written != writer.total {
		_ = os.Remove(partial)
		return "", downloadFailed("truncated")
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		_ = os.Remove(partial)
		return "", checksumMismatch()
	}
	if err := os.Rename(partial, target); err != nil {
		_ = os.Remove(partial)
		return "", downloadFailed("rename")
	}
	return target, nil
}

func (s *Service) fetchChecksums(ctx context.Context) (string, error) {
	s.mu.Lock()
	url := s.checksumURL
	s.mu.Unlock()
	if url == "" {
		return "", checksumMissing()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", checksumMissing()
	}
	request.Header.Set("User-Agent", "Qterm-Updater")
	response, err := s.downloadClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", downloadFailed("checksum-network")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", checksumMissing()
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumBytes))
	if err != nil {
		return "", checksumMissing()
	}
	return string(body), nil
}

func checksumFor(sums, name string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		file := strings.TrimPrefix(strings.TrimSpace(fields[len(fields)-1]), "*")
		file = strings.TrimPrefix(file, "./")
		if len(hash) != 64 || !isHex(hash) {
			continue
		}
		if file == name {
			return hash, true
		}
	}
	return "", false
}

func isHex(value string) bool {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func sanitizeSegment(value string) string {
	builder := strings.Builder{}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '.', char == '-', char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}
	cleaned := strings.Trim(builder.String(), ".")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

type progressWriter struct {
	service  *Service
	release  *Release
	writer   io.Writer
	total    int64
	written  int64
	lastEmit time.Time
}

func (w *progressWriter) Write(buffer []byte) (int, error) {
	written, err := w.writer.Write(buffer)
	w.written += int64(written)
	now := w.service.now()
	if w.lastEmit.IsZero() || now.Sub(w.lastEmit) >= progressInterval {
		w.lastEmit = now
		w.report()
	}
	return written, err
}

func (w *progressWriter) report() {
	percent := 0
	if w.total > 0 {
		percent = int(w.written * 100 / w.total)
		if percent > 100 {
			percent = 100
		}
	}
	release := w.release
	downloaded := w.written
	total := w.total
	w.service.setState(StateDownloading, func(status *Status) {
		status.Release = release
		status.Downloaded = downloaded
		status.Total = total
		status.Percent = percent
		status.MessageKey = ""
	})
}
