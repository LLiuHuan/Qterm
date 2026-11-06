package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path"
	"sync"
)

const resumeFingerprintWindow = 1 << 20

type SFTPResumeManifest struct {
	mu sync.Mutex

	TempPath string

	Size int64

	ChunkSize int64

	Fingerprint string

	Completed map[int64]int64

	ChunkDigests map[int64]string
}

func NewSFTPResumeManifest(tempPath string, size, chunkSize int64, fingerprint string) *SFTPResumeManifest {
	return &SFTPResumeManifest{
		TempPath:     tempPath,
		Size:         size,
		ChunkSize:    chunkSize,
		Fingerprint:  fingerprint,
		Completed:    make(map[int64]int64),
		ChunkDigests: make(map[int64]string),
	}
}

func (m *SFTPResumeManifest) MarkChunkDone(index, written int64, digest string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Completed[index] = written
	if digest != "" {
		m.ChunkDigests[index] = digest
	}
}

func (m *SFTPResumeManifest) Skip() map[int64]struct{} {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Completed) == 0 {
		return nil
	}
	skip := make(map[int64]struct{}, len(m.Completed))
	for index := range m.Completed {
		skip[index] = struct{}{}
	}
	return skip
}

func (m *SFTPResumeManifest) CompletedBytes() int64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, written := range m.Completed {
		total += written
	}
	return total
}

func (m *SFTPResumeManifest) Reset(size int64, chunkSize int64, fingerprint string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Size = size
	m.ChunkSize = chunkSize
	m.Fingerprint = fingerprint
	m.Completed = make(map[int64]int64)
	m.ChunkDigests = make(map[int64]string)
}

func (m *SFTPResumeManifest) EnsurePlan(chunkSize int64) bool {
	if m == nil || chunkSize <= 0 {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ChunkSize == 0 {
		m.ChunkSize = chunkSize
		return true
	}
	return m.ChunkSize == chunkSize
}

func (m *SFTPResumeManifest) MaxWrittenOffset() int64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var maxOffset int64
	for index, written := range m.Completed {
		end := index*m.ChunkSize + written
		if end > maxOffset {
			maxOffset = end
		}
	}
	return maxOffset
}

func (m *SFTPResumeManifest) LastChunk() (index, written int64, digest string, ok bool) {
	if m == nil {
		return 0, 0, "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for candidate, size := range m.Completed {
		if !ok || candidate > index {
			index, written, digest, ok = candidate, size, m.ChunkDigests[candidate], true
		}
	}
	return index, written, digest, ok
}

func resumeChunkDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resumeSourceFingerprint(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open resume source: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat resume source: %w", err)
	}
	hasher := fnv.New64a()
	window := int64(resumeFingerprintWindow)
	if info.Size() < window*2 {
		window = info.Size() / 2
	}
	if window > 0 {
		head := make([]byte, window)
		if _, err := file.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read resume source head: %w", err)
		}
		_, _ = hasher.Write(head)
		tail := make([]byte, window)
		if _, err := file.ReadAt(tail, info.Size()-window); err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read resume source tail: %w", err)
		}
		_, _ = hasher.Write(tail)
	}
	return fmt.Sprintf("%d:%d:%x", info.Size(), info.ModTime().UnixNano(), hasher.Sum64()), nil
}

func verifyResumeTempFile(fs *FileSystem, tempPath string, minimumSize int64) error {
	if fs == nil {
		return ErrFileSystemNotReady
	}
	if tempPath == "" {
		return errors.New("resume temporary path is empty")
	}
	info, err := fs.client.Stat(tempPath)
	if err != nil {
		return fmt.Errorf("stat resume temporary file: %w", err)
	}
	if info.Size() < minimumSize {
		return fmt.Errorf("resume temporary file is too small: got %d, want at least %d", info.Size(), minimumSize)
	}
	return nil
}

func resumeTempSuffix(taskID string) string {
	const maxLen = 40
	out := make([]rune, 0, maxLen)
	for _, r := range taskID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
		if len(out) >= maxLen {
			break
		}
	}
	if len(out) == 0 {
		return "transfer"
	}
	return string(out)
}

func resumeTemporaryPath(remotePath, suffix string) string {
	directory := path.Dir(remotePath)
	base := path.Base(remotePath)
	return path.Join(directory, ".qterm-"+base+"-"+suffix+".part")
}

var resumeManifests sync.Map

func ResumeManifestFor(key, remotePath string, size int64) *SFTPResumeManifest {
	if value, ok := resumeManifests.Load(key); ok {
		if manifest, valid := value.(*SFTPResumeManifest); valid && manifest != nil {
			return manifest
		}
	}
	manifest := NewSFTPResumeManifest(resumeTemporaryPath(remotePath, resumeTempSuffix(key)), size, 0, "")
	actual, _ := resumeManifests.LoadOrStore(key, manifest)
	if existing, valid := actual.(*SFTPResumeManifest); valid && existing != nil {
		return existing
	}
	return manifest
}

func DropResumeManifest(key string) string {
	value, ok := resumeManifests.LoadAndDelete(key)
	if !ok {
		return ""
	}
	manifest, valid := value.(*SFTPResumeManifest)
	if !valid || manifest == nil {
		return ""
	}
	return manifest.TempPath
}

func verifyResumeBeforeResume(f *FileSystem, manifest *SFTPResumeManifest, expectedSize int64) error {
	if manifest == nil {
		return nil
	}

	index, written, digest, ok := manifest.LastChunk()
	if !ok {
		return nil
	}
	if err := verifyResumeTempFile(f, manifest.TempPath, manifest.MaxWrittenOffset()); err != nil {
		return err
	}
	if digest == "" {
		return nil
	}
	offset := index * manifest.ChunkSize
	length := written
	if remaining := expectedSize - offset; remaining < length {
		length = remaining
	}
	actual, err := f.readRemoteChunkDigest(manifest.TempPath, offset, length)
	if err != nil {
		return err
	}
	if actual != digest {
		return errors.New("resume temporary chunk digest mismatch")
	}
	return nil
}
