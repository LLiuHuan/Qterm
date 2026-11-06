package zmodem

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const maxChunk = 64 << 10
const maxDownload = 256 << 20

type LocalFile struct {
	Conflict bool   `json:"conflict"`
	Token    string `json:"token"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

type localFile struct {
	gate           chan struct{}
	file           *os.File
	target         string
	original       os.FileInfo
	size           int64
	offset         int64
	closed         bool
	awaitingChoice bool
}

type Manager struct {
	mu     sync.Mutex
	files  map[string]*localFile
	closed bool
}

func NewManager() *Manager { return &Manager{files: make(map[string]*localFile)} }

func (m *Manager) OpenUpload(ctx context.Context, path string) (LocalFile, error) {
	if err := ctx.Err(); err != nil {
		return LocalFile{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return LocalFile{}, fileError(err)
	}
	if !info.Mode().IsRegular() || info.Size() > 0xffffffff {
		return LocalFile{}, invalidFile()
	}
	f, err := os.Open(path)
	if err != nil {
		return LocalFile{}, fileError(err)
	}
	actual, err := f.Stat()
	if err != nil || !os.SameFile(info, actual) {
		return LocalFile{}, fileError(errors.Join(err, f.Close(), invalidFile()))
	}
	return m.register(ctx, &localFile{file: f, size: info.Size()}, filepath.Base(path))
}

func (m *Manager) CreateDownload(ctx context.Context, path string, size int64) (LocalFile, error) {
	if err := ctx.Err(); err != nil {
		return LocalFile{}, err
	}
	if !filepath.IsAbs(path) || size < 0 || size > maxDownload {
		return LocalFile{}, invalidFile()
	}
	original, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LocalFile{}, fileError(err)
	}
	if original != nil && !original.Mode().IsRegular() {
		return LocalFile{}, invalidFile()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".qterm-zmodem-*")
	if err != nil {
		return LocalFile{}, fileError(err)
	}
	return m.register(ctx, &localFile{file: f, target: path, original: original, size: size, awaitingChoice: original != nil}, filepath.Base(path))
}

func (m *Manager) Read(ctx context.Context, token string, offset int64, length int) ([]byte, error) {
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return nil, err
	}
	defer release()
	if f.target != "" || offset < 0 || offset > f.size || length <= 0 || length > maxChunk {
		return nil, invalidFile()
	}
	data := make([]byte, min(int64(length), f.size-offset))
	n, err := f.file.ReadAt(data, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fileError(err)
	}
	if n != len(data) {
		return nil, fileError(io.ErrUnexpectedEOF)
	}
	return data, nil
}

func (m *Manager) Write(ctx context.Context, token string, offset int64, data []byte) error {
	if len(data) == 0 || len(data) > maxChunk {
		return invalidFile()
	}
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return err
	}
	defer release()
	if f.awaitingChoice || f.target == "" || offset != f.offset || offset+int64(len(data)) > maxDownload {
		return invalidFile()
	}
	n, err := f.file.Write(data)
	f.offset += int64(n)
	if err != nil {
		return fileError(err)
	}
	if n != len(data) {
		return fileError(io.ErrShortWrite)
	}
	return nil
}

func (m *Manager) Finish(ctx context.Context, token string) error {
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return err
	}
	defer release()
	if f.awaitingChoice || f.target == "" || (f.size != 0 && f.offset != f.size) {
		return invalidFile()
	}
	if err := f.file.Sync(); err != nil {
		return fileError(err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := os.Lstat(f.target)
	if f.original == nil {

		if !errors.Is(err, os.ErrNotExist) {
			return invalidFile()
		}
	} else if err != nil || !os.SameFile(f.original, current) || !current.Mode().IsRegular() || current.Size() != f.original.Size() || !current.ModTime().Equal(f.original.ModTime()) {
		return invalidFile()
	}
	if err := f.file.Close(); err != nil {
		return fileError(err)
	}
	temporary := f.file.Name()
	if f.original == nil {
		err = os.Link(temporary, f.target)
		if err == nil {
			err = os.Remove(temporary)
		}
	} else {

		err = os.Rename(temporary, f.target)
	}
	if err != nil {
		return fileError(err)
	}
	f.closed = true
	m.mu.Lock()
	delete(m.files, token)
	m.mu.Unlock()
	return nil
}

func (m *Manager) Release(token string) error {
	m.mu.Lock()
	f := m.files[token]
	delete(m.files, token)
	m.mu.Unlock()
	if f == nil {
		return nil
	}
	f.gate <- struct{}{}
	defer func() { <-f.gate }()
	return fileError(cleanupFile(f))
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	files := m.files
	m.files = make(map[string]*localFile)
	m.mu.Unlock()
	var result error
	for _, f := range files {
		f.gate <- struct{}{}
		result = errors.Join(result, cleanupFile(f))
		<-f.gate
	}
	return fileError(result)
}

func (m *Manager) register(ctx context.Context, f *localFile, name string) (LocalFile, error) {
	f.gate = make(chan struct{}, 1)
	token := rand.Text()
	m.mu.Lock()
	if m.closed || len(m.files) >= 32 || ctx.Err() != nil {
		m.mu.Unlock()
		return LocalFile{}, fileError(errors.Join(invalidFile(), cleanupFile(f)))
	}
	m.files[token] = f
	m.mu.Unlock()
	return LocalFile{Token: token, Name: name, Size: f.size, Conflict: f.awaitingChoice}, nil
}

func (m *Manager) acquire(ctx context.Context, token string) (*localFile, func(), error) {
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
		return nil, nil, invalidFile()
	}
	return f, release, nil
}

func cleanupFile(f *localFile) error {
	if f.closed {
		return nil
	}
	f.closed = true
	err := f.file.Close()
	if errors.Is(err, os.ErrClosed) {
		err = nil
	}
	if f.target != "" {
		removeErr := os.Remove(f.file.Name())
		if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}

func invalidFile() error        { return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "") }
func fileError(err error) error { return errno.Wrap(errno.CodeStoreFailed, "error.storeFailed", err) }

func (m *Manager) ResolveDownload(ctx context.Context, token, action string) (LocalFile, error) {
	f, release, err := m.acquire(ctx, token)
	if err != nil {
		return LocalFile{}, err
	}
	defer release()
	if !f.awaitingChoice || f.target == "" {
		return LocalFile{}, invalidFile()
	}
	switch action {
	case "overwrite":

	case "copy":
		originalTarget := f.target
		found := false
		for number := 1; number <= 10000; number++ {
			if err := ctx.Err(); err != nil {
				return LocalFile{}, err
			}
			candidate := filepath.Join(filepath.Dir(originalTarget), model.TransferCopyName(filepath.Base(originalTarget), number))
			_, err := os.Lstat(candidate)
			if errors.Is(err, os.ErrNotExist) {
				f.target, f.original = candidate, nil
				found = true
				break
			}
			if err != nil {
				return LocalFile{}, fileError(err)
			}
		}
		if !found {
			return LocalFile{}, invalidFile()
		}
	default:
		return LocalFile{}, invalidFile()
	}
	f.awaitingChoice = false
	return LocalFile{Token: token, Name: filepath.Base(f.target), Size: f.size}, nil
}
