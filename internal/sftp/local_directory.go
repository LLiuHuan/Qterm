package sftp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/session"
)

func normalizeDialogPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > localDialogPathLimit || !utf8.ValidString(value) ||
		strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n") || !filepath.IsAbs(value) {
		return "", errno.New(errno.CodeInvalidArg, "sftp.local_path_invalid", "")
	}
	return filepath.Clean(value), nil
}

func readLocalDirectory(ctx context.Context, directory string) ([]LocalDirectoryEntry, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, localDirectoryError(err, "sftp.local_directory_read_failed")
	}
	result := make([]LocalDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if err := localContextReady(ctx); err != nil {
			return nil, err
		}
		name := entry.Name()
		if !isSafeLocalBasename(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		isSymlink := info.Mode()&os.ModeSymlink != 0
		mode := info.Mode().String()
		entryPath := filepath.Join(directory, name)
		if isSymlink {
			target, statErr := os.Stat(entryPath)
			if statErr != nil {

				target = info
			}
			info = target
		}
		entryType := LocalFileTypeOther
		size := int64(0)
		switch {
		case info.IsDir():
			entryType = LocalFileTypeDirectory
		case info.Mode().IsRegular():
			entryType = LocalFileTypeFile
			size = info.Size()
		}
		if size < 0 {
			size = 0
		}
		result = append(result, LocalDirectoryEntry{
			Name:        name,
			Type:        entryType,
			Size:        size,
			SizeText:    model.FormatBytes(uint64(size)),
			ModTime:     info.ModTime().UTC(),
			Mode:        mode,
			DisplayPath: displayLocalPath(filepath.Join(directory, name)),
			IsSymlink:   isSymlink,
		})
	}
	sort.SliceStable(result, func(left, right int) bool {
		leftDirectory := result[left].Type == LocalFileTypeDirectory
		rightDirectory := result[right].Type == LocalFileTypeDirectory
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

func validateLocalDirectory(ctx context.Context, directory string) error {
	if err := localContextReady(ctx); err != nil {
		return err
	}

	info, err := os.Stat(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	if err != nil {
		return localDirectoryError(err, "sftp.local_directory_failed")
	}
	if !info.IsDir() {
		return errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
	}
	return nil
}

func localRootDirectory() (string, error) {
	base, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(base) {
		fallback, fallbackErr := os.Getwd()
		if fallbackErr != nil || !filepath.IsAbs(fallback) {
			return "", errno.Wrap(errno.CodeStoreFailed, "sftp.local_directory_failed", err)
		}
		base = fallback
	}
	return filepath.VolumeName(base) + string(filepath.Separator), nil
}

func localParentDirectory(root, directory string) string {
	if filepath.Clean(directory) == filepath.Clean(root) {
		return ""
	}
	parent := filepath.Dir(directory)

	if parent == directory {
		return ""
	}
	return parent
}

func displayLocalPath(directory string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return directory
	}
	relative, err := filepath.Rel(home, directory)
	if err != nil {
		return directory
	}
	if relative == "." {
		return "~"
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return directory
	}
	return "~" + string(filepath.Separator) + relative
}

func localDirectoryError(err error, fallbackKey string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrPermission):
		return errno.New(errno.CodePermissionDenied, "sftp.local_directory_permission_denied", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	default:
		return errno.Wrap(errno.CodeStoreFailed, fallbackKey, err)
	}
}

func localDirectoryName(directory string) string {
	name := filepath.Base(directory)
	if !isSafeLocalBasename(name) {
		return "本地"
	}
	return name
}

func isSafeLocalBasename(value string) bool {

	return value != "" && value != "." && value != ".." && len(value) <= localEntryNameLimit &&
		utf8.ValidString(value) && strings.IndexByte(value, 0) < 0 &&
		!strings.ContainsAny(value, "/\\\r\n")
}

func newLocalDirectoryToken() (string, error) {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func localContextReady(ctx context.Context) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	return ctx.Err()
}

func validateWorkspaceSession(ctx context.Context, manager *session.SessionManager, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	info, err := manager.Get(ctx, sessionID)
	if err != nil {
		return "", ServiceError(err)
	}
	if info.State != model.SessionStateConnected {
		return "", errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	if info.Kind != model.SessionKindSSH {
		return "", errno.New(errno.CodeInvalidArg, "sftp.unsupported", "")
	}
	return sessionID, nil
}
