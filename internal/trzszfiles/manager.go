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
	gate      chan struct{}
	file      *os.File
	parent    *os.Root
	temporary string
	reserved  *targetReservation
	original  os.FileInfo
	name      string
	rel       string
	offset    int64
	closed    bool
}

type targetReservation struct {
	parent os.FileInfo
	name   string
}

type Manager struct {
	mu       sync.Mutex
	roots    map[string]*os.Root
	files    map[string]*downloadFile
	reserved map[*targetReservation]struct{}
	closed   bool
}

func NewManager() *Manager {
	return &Manager{
		roots:    make(map[string]*os.Root),
		files:    make(map[string]*downloadFile),
		reserved: make(map[*targetReservation]struct{}),
	}
}

func (m *Manager) OpenDirectory(path string) (Directory, error) {
	if !filepath.IsAbs(path) {
		return Directory{}, invalidPath()
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return Directory{}, fileError(err)
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Directory{}, fileError(errors.Join(invalidPath(), root.Close()))
	}
	token := rand.Text()
	m.roots[token] = root
	m.mu.Unlock()
	return Directory{Token: token, Name: filepath.Base(path)}, nil
}

func (m *Manager) ReleaseDirectory(token string) error {
	m.mu.Lock()
	root := m.roots[token]
	delete(m.roots, token)
	m.mu.Unlock()
	if root == nil {
		return nil
	}
	return fileError(root.Close())
}

func (m *Manager) directory(token string) *os.Root {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.roots[token]
}

func (m *Manager) CreateDirectory(ctx context.Context, dirToken, relativePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	segments, err := splitRelativePath(relativePath)
	if err != nil {
		return err
	}
	root := m.directory(dirToken)
	if root == nil {
		return invalidPath()
	}
	return fileError(root.MkdirAll(filepath.Join(segments...), 0o755))
}

func (m *Manager) OpenDownload(ctx context.Context, dirToken, relativePath string, overwrite bool) (File, error) {
	if err := ctx.Err(); err != nil {
		return File{}, err
	}
	segments, err := splitRelativePath(relativePath)
	if err != nil {
		return File{}, err
	}
	root := m.directory(dirToken)
	if root == nil {
		return File{}, invalidPath()
	}
	relative := filepath.Join(segments...)
	parentPath := filepath.Dir(relative)
	if err := root.MkdirAll(parentPath, 0o755); err != nil {
		return File{}, fileError(err)
	}
	parent, err := root.OpenRoot(parentPath)
	if err != nil {
		return File{}, fileError(err)
	}
	reserved, original, err := m.reserveTarget(parent, filepath.Base(relative), overwrite)
	if err != nil {
		return File{}, fileError(errors.Join(err, parent.Close()))
	}

	temporary := ".qterm-trzsz-" + rand.Text()
	file, err := parent.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		m.forget("", reserved)
		return File{}, fileError(errors.Join(err, parent.Close()))
	}
	return m.register(ctx, &downloadFile{
		gate:      make(chan struct{}, 1),
		file:      file,
		parent:    parent,
		temporary: temporary,
		reserved:  reserved,
		original:  original,
		name:      reserved.name,
		rel:       filepath.ToSlash(filepath.Join(parentPath, reserved.name)),
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
	if err := f.file.Close(); err != nil {
		return File{}, fileError(err)
	}
	if f.original == nil {

		err = f.parent.Link(f.temporary, f.name)
		if err != nil && !errors.Is(err, os.ErrExist) {

			if copyErr := copyExclusive(ctx, f); copyErr != nil {
				err = errors.Join(err, copyErr)
			} else {
				err = nil
			}
		}
	} else {
		current, statErr := f.parent.Lstat(f.name)
		if statErr != nil || !current.Mode().IsRegular() || !os.SameFile(f.original, current) ||
			current.Size() != f.original.Size() || !current.ModTime().Equal(f.original.ModTime()) {
			return File{}, invalidPath()
		}

		err = f.parent.Rename(f.temporary, f.name)
		if err == nil {
			f.temporary = ""
		}
	}
	if err != nil {
		return File{}, fileError(err)
	}
	m.forget(token, f.reserved)
	if err := cleanup(f); err != nil {
		return File{}, fileError(err)
	}
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
	if f.closed {
		return nil
	}
	defer m.forget(token, f.reserved)
	return fileError(cleanup(f))
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	files, roots := m.files, m.roots
	m.files, m.roots, m.reserved = nil, nil, nil
	m.mu.Unlock()
	var result error
	for _, f := range files {
		f.gate <- struct{}{}
		result = errors.Join(result, cleanup(f))
		<-f.gate
	}
	for _, root := range roots {
		result = errors.Join(result, root.Close())
	}
	return fileError(result)
}

func (m *Manager) register(ctx context.Context, f *downloadFile) (File, error) {
	m.mu.Lock()
	if m.closed || len(m.files) >= maxFiles || ctx.Err() != nil {
		delete(m.reserved, f.reserved)
		m.mu.Unlock()
		return File{}, fileError(errors.Join(invalidPath(), cleanup(f)))
	}
	token := rand.Text()
	m.files[token] = f
	m.mu.Unlock()
	return File{Token: token, Name: f.name, RelativePath: f.rel}, nil
}

func (m *Manager) forget(token string, reserved *targetReservation) {
	m.mu.Lock()
	delete(m.files, token)
	delete(m.reserved, reserved)
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

func (m *Manager) reserveTarget(parent *os.Root, name string, overwrite bool) (*targetReservation, os.FileInfo, error) {
	parentInfo, err := parent.Stat(".")
	if err != nil {
		return nil, nil, err
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for index := 0; index <= 999; index++ {
		candidate := name
		if index > 0 {
			candidate = stem + " (" + strconv.Itoa(index) + ")" + ext
		}
		info, err := parent.Lstat(candidate)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
		if info != nil {
			if !overwrite {
				continue
			}
			if !info.Mode().IsRegular() {
				return nil, nil, invalidPath()
			}
		}
		reservation := &targetReservation{parent: parentInfo, name: candidate}
		m.mu.Lock()
		reserved := false
		for target := range m.reserved {
			if strings.EqualFold(target.name, candidate) && os.SameFile(target.parent, parentInfo) {
				reserved = true
				break
			}
		}
		closed := m.closed
		if !closed && !reserved {
			m.reserved[reservation] = struct{}{}
		}
		m.mu.Unlock()
		if closed || (reserved && overwrite) {
			return nil, nil, invalidPath()
		}
		if !reserved {
			return reservation, info, nil
		}
	}
	return nil, nil, invalidPath()
}

type downloadCopyReader struct {
	context.Context
	file *os.File
}

func (r downloadCopyReader) Read(buffer []byte) (int, error) {
	if err := r.Context.Err(); err != nil {
		return 0, err
	}
	return r.file.Read(buffer)
}

func copyExclusive(ctx context.Context, f *downloadFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := f.parent.Open(f.temporary)
	if err != nil {
		return err
	}
	target, err := f.parent.OpenFile(f.name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.Join(err, source.Close())
	}
	created, statErr := target.Stat()
	var copyErr error
	if statErr == nil {
		count, err := io.Copy(target, downloadCopyReader{Context: ctx, file: source})
		copyErr = err
		if copyErr == nil && count != f.offset {
			copyErr = io.ErrUnexpectedEOF
		}
		if copyErr == nil {
			copyErr = errors.Join(ctx.Err(), target.Sync())
		}
	}
	err = errors.Join(statErr, copyErr, source.Close(), target.Close())
	if err == nil {
		return nil
	}

	if current, statErr := f.parent.Lstat(f.name); statErr == nil && created != nil && os.SameFile(created, current) {
		err = errors.Join(err, f.parent.Remove(f.name))
	}
	return err
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
	if f.temporary != "" {
		removeErr := f.parent.Remove(f.temporary)
		if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}
	return errors.Join(err, f.parent.Close())
}

func invalidPath() error { return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "") }

func fileError(err error) error { return errno.Wrap(errno.CodeStoreFailed, "error.storeFailed", err) }
