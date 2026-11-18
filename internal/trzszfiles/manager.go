package trzszfiles

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"changeme/internal/errno"
)

const maxChunk = 1 << 20

const maxFiles = 32

const maxRelPathLength = 4096

type Directory struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type File struct {
	Token        string `json:"token"`
	Name         string `json:"name"`
	RelativePath string `json:"relativePath"`
}

type downloadFile struct {
	gate   chan struct{}
	file   *os.File
	target string
	name   string
	rel    string
	offset int64
	closed bool
}

type Manager struct {
	mu     sync.Mutex
	roots  map[string]string
	files  map[string]*downloadFile
	closed bool
}

func NewManager() *Manager {
	return &Manager{roots: make(map[string]string), files: make(map[string]*downloadFile)}
}

func (m *Manager) OpenDirectory(path string) (Directory, error) {
	if !filepath.IsAbs(path) {
		return Directory{}, invalidPath()
	}
	info, err := os.Stat(path)
	if err != nil {
		return Directory{}, fileError(err)
	}
	if !info.IsDir() {
		return Directory{}, invalidPath()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Directory{}, invalidPath()
	}
	token := rand.Text()
	m.roots[token] = path
	return Directory{Token: token, Name: filepath.Base(path)}, nil
}

func (m *Manager) CreateDirectory(ctx context.Context, dirToken, relativePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	segments, err := splitRelativePath(relativePath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	root, ok := m.roots[dirToken]
	closed := m.closed
	m.mu.Unlock()
	if closed || !ok {
		return invalidPath()
	}
	target := filepath.Join(append([]string{root}, segments...)...)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fileError(err)
	}
	return nil
}

func (m *Manager) OpenDownload(ctx context.Context, dirToken, relativePath string, overwrite bool) (File, error) {
	if err := ctx.Err(); err != nil {
		return File{}, err
	}
	segments, err := splitRelativePath(relativePath)
	if err != nil {
		return File{}, err
	}
	m.mu.Lock()
	root, ok := m.roots[dirToken]
	closed := m.closed
	m.mu.Unlock()
	if closed || !ok {
		return File{}, invalidPath()
	}
	target := filepath.Join(append([]string{root}, segments...)...)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return File{}, fileError(err)
	}

	name := filepath.Base(target)
	if !overwrite {
		target, name = uniqueTarget(target)
	}
	temporary, err := os.CreateTemp(parent, ".qterm-trzsz-*")
	if err != nil {
		return File{}, fileError(err)
	}
	rel := strings.Join(segments, "/")
	return m.register(ctx, &downloadFile{
		gate:   make(chan struct{}, 1),
		file:   temporary,
		target: target,
		name:   name,
		rel:    rel,
	})
}

func (m *Manager) Write(ctx context.Context, token string, offset int64, data []byte) error {
	if offset < 0 || len(data) > maxChunk {
		return invalidPath()
	}
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return err
	}
	defer release()
	if f.offset != offset {
		return invalidPath()
	}
	written, err := f.file.Write(data)
	f.offset += int64(written)
	if err != nil {
		return fileError(err)
	}
	if written != len(data) {
		return fileError(io.ErrShortWrite)
	}
	return nil
}

func (m *Manager) Commit(ctx context.Context, token string) (File, error) {
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return File{}, err
	}
	defer release()
	if err := f.file.Sync(); err != nil {
		return File{}, fileError(err)
	}
	if err := ctx.Err(); err != nil {
		return File{}, err
	}
	temporary := f.file.Name()
	if err := f.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return File{}, fileError(err)
	}

	if err := os.Rename(temporary, f.target); err != nil {
		if removeErr := os.Remove(f.target); removeErr == nil {
			err = os.Rename(temporary, f.target)
		}
		if err != nil {
			return File{}, fileError(err)
		}
	}
	f.closed = true
	m.forget(token)
	return File{Name: f.name, RelativePath: f.rel}, nil
}

func (m *Manager) Abort(token string) error {
	m.mu.Lock()
	f := m.files[token]
	delete(m.files, token)
	m.mu.Unlock()
	if f == nil {
		return nil
	}
	f.gate <- struct{}{}
	defer func() { <-f.gate }()
	return fileError(cleanup(f))
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	files := m.files
	m.files = make(map[string]*downloadFile)
	m.roots = make(map[string]string)
	m.mu.Unlock()
	var result error
	for _, f := range files {
		f.gate <- struct{}{}
		result = errors.Join(result, cleanup(f))
		<-f.gate
	}
	return fileError(result)
}

func (m *Manager) register(ctx context.Context, f *downloadFile) (File, error) {
	m.mu.Lock()
	if m.closed || len(m.files) >= maxFiles || ctx.Err() != nil {
		m.mu.Unlock()
		return File{}, fileError(errors.Join(invalidPath(), cleanup(f)))
	}
	token := rand.Text()
	m.files[token] = f
	m.mu.Unlock()
	return File{Token: token, Name: f.name, RelativePath: f.rel}, nil
}

func (m *Manager) forget(token string) {
	m.mu.Lock()
	delete(m.files, token)
	m.mu.Unlock()
}

func (m *Manager) acquire(ctx context.Context, token string) (*downloadFile, func(), error) {
	m.mu.Lock()
	f := m.files[token]
	m.mu.Unlock()
	if f == nil {
		return nil, nil, errno.New(errno.CodeNotFound, "error.notFound", "")
	}
	select {
	case f.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	release := func() { <-f.gate }
	if f.closed || ctx.Err() != nil {
		release()
		return nil, nil, invalidPath()
	}
	return f, release, nil
}

func splitRelativePath(relativePath string) ([]string, error) {
	if relativePath == "" || len(relativePath) > maxRelPathLength || strings.ContainsRune(relativePath, 0) {
		return nil, invalidPath()
	}
	normalized := strings.ReplaceAll(relativePath, "\\", "/")
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(normalized) {
		return nil, invalidPath()
	}
	raw := strings.Split(normalized, "/")
	segments := make([]string, 0, len(raw))
	for _, segment := range raw {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." {
			return nil, invalidPath()
		}

		if strings.ContainsAny(segment, ":<>|\"?*") {
			return nil, invalidPath()
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		return nil, invalidPath()
	}
	return segments, nil
}

func uniqueTarget(target string) (string, string) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for index := 1; index <= 999; index++ {
		candidate := filepath.Join(dir, stem+" ("+strconv.Itoa(index)+")"+ext)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, filepath.Base(candidate)
		}
	}
	return target, base
}

func cleanup(f *downloadFile) error {
	if f.closed {
		return nil
	}
	f.closed = true
	err := f.file.Close()
	if errors.Is(err, os.ErrClosed) {
		err = nil
	}
	if temporary := f.file.Name(); temporary != "" {
		removeErr := os.Remove(temporary)
		if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}

func invalidPath() error { return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "") }

func fileError(err error) error { return errno.Wrap(errno.CodeStoreFailed, "error.storeFailed", err) }
