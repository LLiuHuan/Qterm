package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
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
		slog.Warn("update staging directory create failed",
			"event", "update.download.mkdir_failed",
			"dir", dir,
			"error", err,
		)
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
		slog.Warn("update checksum missing for asset",
			"event", "update.download.checksum_missing",
			"asset", release.Asset.Name,
			"version", release.Version,
		)
		return "", checksumMissing()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.Asset.URL, nil)
	if err != nil {
		slog.Warn("update download request invalid", "event", "update.download.request_invalid", "asset", release.Asset.Name)
		return "", downloadFailed("request")
	}
	request.Header.Set("User-Agent", "Qterm-Updater")
	response, err := s.downloadClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		slog.Warn("update download request failed",
			"event", "update.download.request_failed",
			"asset", release.Asset.Name,
			"error", err,
		)
		return "", downloadFailed("network")
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		slog.Warn("update download rejected",
			"event", "update.download.bad_status",
			"asset", release.Asset.Name,
			"status", response.StatusCode,
		)
		return "", downloadFailed("HTTP " + strconv.Itoa(response.StatusCode))
	}

	body := newStallGuard(response.Body, downloadStallTimeout)
	defer func() { _ = body.Close() }()

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
	written, copyErr := io.Copy(writer, body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(partial)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if body.Stalled() {

			slog.Warn("update download stalled",
				"event", "update.download.stalled",
				"asset", release.Asset.Name,
				"bytes", written,
			)
			return "", downloadFailed("stalled")
		}
		slog.Warn("update download copy failed",
			"event", "update.download.copy_failed",
			"asset", release.Asset.Name,
			"bytes", written,
			"error", copyErr,
		)
		return "", downloadFailed("copy")
	}
	if closeErr != nil {
		_ = os.Remove(partial)
		slog.Warn("update download close failed",
			"event", "update.download.close_failed",
			"asset", release.Asset.Name,
			"error", closeErr,
		)
		return "", downloadFailed("close")
	}
	if writer.total > 0 && written != writer.total {
		_ = os.Remove(partial)
		slog.Warn("update download truncated",
			"event", "update.download.truncated",
			"asset", release.Asset.Name,
			"bytes", written,
			"total", writer.total,
		)
		return "", downloadFailed("truncated")
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		_ = os.Remove(partial)
		slog.Warn("update download checksum mismatch",
			"event", "update.download.checksum_mismatch",
			"asset", release.Asset.Name,
			"bytes", written,
		)
		return "", checksumMismatch()
	}
	if err := os.Rename(partial, target); err != nil {
		_ = os.Remove(partial)
		slog.Warn("update download rename failed",
			"event", "update.download.rename_failed",
			"target", target,
			"error", err,
		)
		return "", downloadFailed("rename")
	}
	return target, nil
}

func (s *Service) fetchChecksums(ctx context.Context) (string, error) {
	s.mu.Lock()
	checksumURL := s.checksumURL
	signatureURL := s.signatureURL
	s.mu.Unlock()
	if checksumURL == "" {
		slog.Warn("update checksum asset missing", "event", "update.signature.checksum_asset_missing")
		return "", checksumMissing()
	}
	if signatureURL == "" {
		slog.Warn("update signature asset missing", "event", "update.signature.asset_missing")
		return "", signatureMissing()
	}
	sums, err := s.fetchText(ctx, checksumURL, maxChecksumBytes, checksumMissing)
	if err != nil {
		return "", err
	}
	signature, err := s.fetchText(ctx, signatureURL, maxSignatureBytes, signatureMissing)
	if err != nil {
		return "", err
	}
	if err := VerifyChecksumsSignature([]byte(sums), []byte(signature)); err != nil {

		key, detail := errorFields(err)
		slog.Error("update checksums signature rejected",
			"event", "update.signature.rejected",
			"error_key", key,
			"detail", detail,
		)
		return "", err
	}
	return sums, nil
}

func (s *Service) fetchText(ctx context.Context, url string, limit int64, fallback func() error) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fallback()
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
		return "", fallback()
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit))
	if err != nil {
		return "", fallback()
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

	lastLog int64
}

func (w *progressWriter) Write(buffer []byte) (int, error) {
	written, err := w.writer.Write(buffer)
	w.written += int64(written)
	now := w.service.now()
	if w.lastEmit.IsZero() || now.Sub(w.lastEmit) >= progressInterval {
		w.lastEmit = now
		w.report()
	}
	w.logProgress()
	return written, err
}

func (w *progressWriter) logProgress() {
	step := int64(1 << 20)
	if w.total > 0 && w.total/10 > step {
		step = w.total / 10
	}
	if w.written-w.lastLog < step && w.written != w.total {
		return
	}
	w.lastLog = w.written
	percent := 0
	if w.total > 0 {
		percent = int(w.written * 100 / w.total)
	}
	slog.Debug("update download progress",
		"event", "update.download.progress",
		"asset", w.release.Asset.Name,
		"bytes", w.written,
		"total", w.total,
		"percent", percent,
	)
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
