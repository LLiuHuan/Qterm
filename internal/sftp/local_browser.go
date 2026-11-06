package sftp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
)

const (
	localDirectoryTokenLimit = 512
	localEntryNameLimit      = 255

	localDialogPathLimit = 4096
)

type LocalFileType string

const (
	LocalFileTypeFile      LocalFileType = "file"
	LocalFileTypeDirectory LocalFileType = "directory"
	LocalFileTypeOther     LocalFileType = "other"
)

type LocalDirectoryEntry struct {
	Name        string        `json:"name"`
	Type        LocalFileType `json:"type"`
	Size        int64         `json:"size"`
	SizeText    string        `json:"sizeText"`
	ModTime     time.Time     `json:"modTime"`
	Mode        string        `json:"mode"`
	DisplayPath string        `json:"displayPath"`
	IsSymlink   bool          `json:"isSymlink"`
}

type LocalDirectorySegment struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type LocalDirectory struct {
	Token       string                  `json:"token"`
	DisplayPath string                  `json:"displayPath"`
	Name        string                  `json:"name"`
	ParentToken string                  `json:"parentToken"`
	Segments    []LocalDirectorySegment `json:"segments"`
	Entries     []LocalDirectoryEntry   `json:"entries"`
}

type localDirectoryToken struct {
	path     string
	lastUsed uint64
}

type localDirectoryBrowser struct {
	mu             sync.Mutex
	tokens         map[string]localDirectoryToken
	sequence       uint64
	startDirectory string
}

func newLocalDirectoryBrowser() *localDirectoryBrowser {
	return &localDirectoryBrowser{tokens: make(map[string]localDirectoryToken)}
}

func (s *Service) OpenSFTPLocalDirectory(ctx context.Context, token, child string) (LocalDirectory, error) {
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return LocalDirectory{}, err
	}
	return browser.open(ctx, token, child)
}

type localDirectoryPicker interface{ ChooseLocalDirectory() (string, error) }

type SFTPLocalDirectorySelectionResult struct {
	Canceled  bool            `json:"canceled"`
	Directory *LocalDirectory `json:"directory"`
}

func (s *Service) SelectSFTPLocalDirectory(ctx context.Context) (SFTPLocalDirectorySelectionResult, error) {
	result := SFTPLocalDirectorySelectionResult{}
	if err := localContextReady(ctx); err != nil {
		return result, err
	}
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return result, err
	}
	_, _, dialogs, err := s.transferDependencies()
	if err != nil {
		return result, err
	}
	picker, ok := dialogs.(localDirectoryPicker)
	if !ok {
		return result, errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}
	selected, err := picker.ChooseLocalDirectory()
	if err != nil {
		return result, errno.New(errno.CodeUnknown, "sftp.open_dialog_failed", "")
	}
	if selected == "" {
		result.Canceled = true
		return result, nil
	}
	directory, err := browser.openDialogDirectory(ctx, selected)
	if err != nil {
		return result, err
	}
	result.Directory = &directory
	return result, nil
}

func (b *localDirectoryBrowser) open(ctx context.Context, token, child string) (LocalDirectory, error) {
	if err := localContextReady(ctx); err != nil {
		return LocalDirectory{}, err
	}
	root, err := localRootDirectory()
	if err != nil {
		return LocalDirectory{}, err
	}
	directory := b.startPath()
	if token != "" {
		directory, err = b.lookup(token)
		if err != nil {
			return LocalDirectory{}, err
		}
	}
	if child != "" {
		if !isSafeLocalBasename(child) {
			return LocalDirectory{}, errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
		}
		directory = filepath.Join(directory, child)
	}
	return b.describe(ctx, root, directory)
}

func (b *localDirectoryBrowser) openDialogDirectory(ctx context.Context, selected string) (LocalDirectory, error) {
	directory, err := normalizeDialogPath(selected)
	if err != nil {
		return LocalDirectory{}, err
	}
	root, err := localRootDirectory()
	if err != nil {
		return LocalDirectory{}, err
	}
	return b.describe(ctx, root, directory)
}

func (b *localDirectoryBrowser) describe(ctx context.Context, root, directory string) (LocalDirectory, error) {
	if err := validateLocalDirectory(ctx, directory); err != nil {
		return LocalDirectory{}, err
	}
	entries, err := readLocalDirectory(ctx, directory)
	if err != nil {
		return LocalDirectory{}, err
	}
	currentToken, err := b.issue(directory)
	if err != nil {
		return LocalDirectory{}, err
	}
	segments, err := b.pathSegments(root, directory)
	if err != nil {
		return LocalDirectory{}, err
	}
	parentToken := ""
	if parent := localParentDirectory(root, directory); parent != "" {
		parentToken, err = b.issue(parent)
		if err != nil {
			return LocalDirectory{}, err
		}
	}
	return LocalDirectory{
		Token:       currentToken,
		DisplayPath: displayLocalPath(directory),
		Name:        localDirectoryName(directory),
		ParentToken: parentToken,
		Segments:    segments,
		Entries:     entries,
	}, nil
}

func (b *localDirectoryBrowser) resolveDirectory(ctx context.Context, token string) (string, error) {
	if err := localContextReady(ctx); err != nil {
		return "", err
	}
	if token == "" {
		return "", errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	directory, err := b.lookup(token)
	if err != nil {
		return "", err
	}
	if err := validateLocalDirectory(ctx, directory); err != nil {
		return "", err
	}
	return directory, nil
}

func (b *localDirectoryBrowser) resolveFile(ctx context.Context, directoryToken, name string) (string, string, int64, error) {
	directory, err := b.resolveDirectory(ctx, directoryToken)
	if err != nil {
		return "", "", 0, err
	}
	if !isSafeLocalBasename(name) {
		return "", "", 0, errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	localPath := filepath.Join(directory, name)
	if err := localContextReady(ctx); err != nil {
		return "", "", 0, err
	}
	info, err := os.Lstat(localPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", 0, errno.New(errno.CodeNotFound, "sftp.local_source_missing", "")
	}
	if err != nil {
		return "", "", 0, errno.Wrap(errno.CodeStoreFailed, "sftp.local_source_failed", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", 0, errno.New(errno.CodeInvalidArg, "sftp.upload_regular_only", "")
	}
	return localPath, name, info.Size(), nil
}

func (b *localDirectoryBrowser) lookup(token string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, ok := b.tokens[token]
	if !ok || token == "" {
		return "", errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	b.sequence++
	value.lastUsed = b.sequence
	b.tokens[token] = value
	return value.path, nil
}

func (b *localDirectoryBrowser) issue(directory string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sequence++
	for token, value := range b.tokens {
		if value.path == directory {
			value.lastUsed = b.sequence
			b.tokens[token] = value
			return token, nil
		}
	}
	if len(b.tokens) >= localDirectoryTokenLimit {
		var oldestToken string
		var oldestUse uint64
		for token, value := range b.tokens {
			if oldestToken == "" || value.lastUsed < oldestUse {
				oldestToken = token
				oldestUse = value.lastUsed
			}
		}
		delete(b.tokens, oldestToken)
	}
	for {
		token, err := newLocalDirectoryToken()
		if err != nil {
			return "", errno.Wrap(errno.CodeUnknown, "sftp.local_directory_failed", err)
		}
		if _, exists := b.tokens[token]; exists {
			continue
		}
		b.tokens[token] = localDirectoryToken{path: directory, lastUsed: b.sequence}
		return token, nil
	}
}

func (b *localDirectoryBrowser) clear() {
	b.mu.Lock()
	clear(b.tokens)
	b.sequence = 0
	b.mu.Unlock()
}

func (b *localDirectoryBrowser) setStartDirectory(value string) error {
	path, err := configuredStartDirectory(value)
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "sftp.local_directory_failed", "")
	}
	if err := validateLocalDirectory(context.Background(), path); err != nil {
		return err
	}
	b.mu.Lock()
	b.startDirectory = path
	b.mu.Unlock()
	return nil
}

func (b *localDirectoryBrowser) startPath() string {
	b.mu.Lock()
	value := b.startDirectory
	b.mu.Unlock()
	if value != "" {
		return value
	}
	home, err := configuredStartDirectory("")
	if err != nil {
		return filepath.VolumeName(string(filepath.Separator)) + string(filepath.Separator)
	}
	return home
}

func (b *localDirectoryBrowser) pathSegments(root, directory string) ([]LocalDirectorySegment, error) {
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
	}
	rootToken, err := b.issue(root)
	if err != nil {
		return nil, err
	}
	segments := []LocalDirectorySegment{{Name: localDirectoryName(root), Token: rootToken}}
	if relative == "." {
		return segments, nil
	}
	current := filepath.Clean(root)
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		token, err := b.issue(current)
		if err != nil {
			return nil, err
		}
		segments = append(segments, LocalDirectorySegment{Name: part, Token: token})
	}
	return segments, nil
}
