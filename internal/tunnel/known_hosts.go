package tunnel

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"changeme/internal/model"
)

var (
	ErrInvalidKnownHostsPath = errors.New("invalid known_hosts path")
	ErrHostKeyUnknown        = errors.New("SSH host key is unknown")
	ErrHostKeyChanged        = errors.New("SSH host key changed")
)

type HostKeyIssue = model.HostKeyIssue

const (
	HostKeyIssueUnknown = model.HostKeyIssueUnknown
	HostKeyIssueChanged = model.HostKeyIssueChanged
)

type HostKeyChallenge = model.HostKeyChallenge

type HostKeyError struct {
	Challenge HostKeyChallenge
	key       ssh.PublicKey
}

func (e *HostKeyError) Error() string {
	if e == nil {
		return "SSH host-key verification failed"
	}
	return fmt.Sprintf("SSH host key %s for %s", e.Challenge.Issue, e.Challenge.Address)
}

func (e *HostKeyError) Unwrap() error {
	if e != nil && e.Challenge.Issue == HostKeyIssueChanged {
		return ErrHostKeyChanged
	}
	return ErrHostKeyUnknown
}

type HostKeyVerifier struct {
	mu            sync.Mutex
	managedPath   string
	readOnlyFiles []string
}

func NewHostKeyVerifier(managedPath string, readOnlyFiles ...string) (*HostKeyVerifier, error) {
	managedPath = strings.TrimSpace(managedPath)
	if managedPath == "" {
		return nil, ErrInvalidKnownHostsPath
	}
	files := make([]string, 0, len(readOnlyFiles))
	for _, path := range readOnlyFiles {
		path = strings.TrimSpace(path)
		if path != "" && path != managedPath {
			files = append(files, path)
		}
	}
	return &HostKeyVerifier{managedPath: managedPath, readOnlyFiles: files}, nil
}

func (v *HostKeyVerifier) Callback() ssh.HostKeyCallback {
	return v.Verify
}

func (v *HostKeyVerifier) Verify(hostname string, remote net.Addr, key ssh.PublicKey) error {
	if v == nil || v.managedPath == "" || key == nil {
		return ErrInvalidKnownHostsPath
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.verifyLocked(hostname, remote, key)
}

func (v *HostKeyVerifier) TrustUnknown(challenge *HostKeyError) error {
	return v.TrustUnknownBatch([]*HostKeyError{challenge})
}

func (v *HostKeyVerifier) TrustUnknownBatch(challenges []*HostKeyError) error {
	if v == nil || v.managedPath == "" {
		return ErrInvalidKnownHostsPath
	}
	if len(challenges) == 0 {

		return nil
	}

	if len(challenges) > 1 {
		for _, challenge := range challenges {
			if challenge != nil && challenge.Challenge.Issue == HostKeyIssueChanged {
				return ErrHostKeyChanged
			}
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	type trustedKey struct {
		address string
		key     ssh.PublicKey
	}
	toWrite := make([]trustedKey, 0, len(challenges))
	for _, challenge := range challenges {
		if challenge == nil || challenge.key == nil {
			return ErrInvalidKnownHostsPath
		}
		if challenge.Challenge.Issue != HostKeyIssueUnknown && challenge.Challenge.Issue != HostKeyIssueChanged {
			return ErrHostKeyUnknown
		}

		err := v.verifyLocked(
			challenge.Challenge.Address,
			verifierAddr(challenge.Challenge.Address),
			challenge.key,
		)
		if err == nil {
			continue
		}
		var current *HostKeyError
		if !errors.As(err, &current) ||
			(current.Challenge.Issue != HostKeyIssueUnknown && current.Challenge.Issue != HostKeyIssueChanged) {
			return err
		}
		toWrite = append(toWrite, trustedKey{address: challenge.Challenge.Address, key: challenge.key})
	}
	if len(toWrite) == 0 {
		return nil
	}
	if err := ensureKnownHostsFile(v.managedPath); err != nil {
		return err
	}
	var lines strings.Builder
	for _, item := range toWrite {
		normalized := knownhosts.Normalize(item.address)
		hashed := knownhosts.HashHostname(normalized)
		lines.WriteString(knownhosts.Line([]string{hashed}, item.key))
		lines.WriteByte('\n')
	}
	data := []byte(lines.String())

	file, err := os.OpenFile(v.managedPath, os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open managed known_hosts: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return fmt.Errorf("inspect managed known_hosts: %w", statErr)
	}
	originalSize := info.Size()
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	syncErr := error(nil)
	if writeErr == nil {
		syncErr = file.Sync()
	}
	if writeErr != nil || syncErr != nil {

		truncateErr := file.Truncate(originalSize)
		if truncateErr == nil {
			truncateErr = file.Sync()
		}
		closeErr := file.Close()
		switch {
		case truncateErr != nil:
			return fmt.Errorf("append managed known_hosts: %v (rollback failed: %v, close: %v)", writeErrOrSync(writeErr, syncErr), truncateErr, closeErr)
		case writeErr != nil:
			return fmt.Errorf("append managed known_hosts: %w", writeErr)
		default:
			return fmt.Errorf("sync managed known_hosts: %w", syncErr)
		}
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("close managed known_hosts: %w", closeErr)
	}
	return nil
}

func writeErrOrSync(writeErr, syncErr error) error {
	if writeErr != nil {
		return writeErr
	}
	return syncErr
}

func (v *HostKeyVerifier) verifyLocked(hostname string, remote net.Addr, key ssh.PublicKey) error {
	files, err := v.existingFilesLocked()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return newHostKeyError(hostname, key, nil)
	}
	callback, err := knownhosts.New(files...)
	if err != nil {
		return fmt.Errorf("load known_hosts: %w", err)
	}
	if err := callback(hostname, remote, key); err == nil {
		return nil
	} else {
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return fmt.Errorf("verify known_hosts: %w", err)
		}
		return newHostKeyError(hostname, key, keyErr.Want)
	}
}

func (v *HostKeyVerifier) existingFilesLocked() ([]string, error) {
	paths := append([]string{v.managedPath}, v.readOnlyFiles...)
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return nil, ErrInvalidKnownHostsPath
			}
			files = append(files, path)
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("inspect known_hosts: %w", err)
		}
	}
	return files, nil
}

func NewHostKeyError(challenge HostKeyChallenge, key ssh.PublicKey) (*HostKeyError, error) {
	if key == nil {
		return nil, ErrHostKeyUnknown
	}
	encoded := key.Marshal()
	ownedKey, err := ssh.ParsePublicKey(encoded)
	if err != nil {
		return nil, fmt.Errorf("copy SSH host key: %w", err)
	}
	return &HostKeyError{Challenge: challenge, key: ownedKey}, nil
}

func newHostKeyError(hostname string, key ssh.PublicKey, want []knownhosts.KnownKey) error {
	encoded := key.Marshal()
	ownedKey, err := ssh.ParsePublicKey(encoded)
	if err != nil {
		return fmt.Errorf("copy SSH host key: %w", err)
	}
	challenge := HostKeyChallenge{
		Address:           hostname,
		Algorithm:         key.Type(),
		SHA256Fingerprint: ssh.FingerprintSHA256(key),
		Issue:             HostKeyIssueUnknown,
	}
	if len(want) > 0 {
		challenge.Issue = HostKeyIssueChanged
		challenge.TrustedFingerprints = make([]string, 0, len(want))
		for _, known := range want {
			challenge.TrustedFingerprints = append(
				challenge.TrustedFingerprints,
				ssh.FingerprintSHA256(known.Key),
			)
		}
	}
	return &HostKeyError{Challenge: challenge, key: ownedKey}
}

func ensureKnownHostsFile(path string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create known_hosts directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("restrict known_hosts directory: %w", err)
		}
	}
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ErrInvalidKnownHostsPath
		}
	case errors.Is(err, fs.ErrNotExist):
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return fmt.Errorf("create managed known_hosts: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("close managed known_hosts: %w", closeErr)
		}
	default:
		return fmt.Errorf("inspect managed known_hosts: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("restrict managed known_hosts: %w", err)
		}
	}
	return nil
}

type verifierAddr string

func (a verifierAddr) Network() string { return "known-hosts" }

func (a verifierAddr) String() string { return string(a) }

func (v *HostKeyVerifier) ManagedPath() string {
	if v == nil {
		return ""
	}
	return v.managedPath
}

func (v *HostKeyVerifier) List() ([]model.KnownHostEntry, error) {
	if v == nil || v.managedPath == "" {
		return nil, ErrInvalidKnownHostsPath
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entries := make([]model.KnownHostEntry, 0, 16)
	if err := appendKnownHostFile(&entries, v.managedPath, model.KnownHostSourceManaged); err != nil {
		return nil, err
	}
	for _, path := range v.readOnlyFiles {
		if err := appendKnownHostFile(&entries, path, model.KnownHostSourceSystem); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func appendKnownHostFile(entries *[]model.KnownHostEntry, path, source string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read known_hosts: %w", err)
	}
	for index, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, ok := parseKnownHostLine(line, source, index+1)
		if !ok {
			continue
		}
		if source == model.KnownHostSourceManaged {
			entry.Editable = true
		}
		*entries = append(*entries, entry)
	}
	return nil
}

func parseKnownHostLine(line, source string, lineNumber int) (model.KnownHostEntry, bool) {
	entry := model.KnownHostEntry{Source: source, Line: lineNumber}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return entry, false
	}
	if strings.HasPrefix(fields[0], "@") {
		if len(fields) < 2 {
			return entry, false
		}
		entry.Marker = fields[0]
		fields = fields[1:]
	}
	if len(fields) < 2 {
		return entry, false
	}
	entry.Host = fields[0]
	entry.KeyType = fields[1]
	if len(fields) >= 3 {
		if key, err := ssh.ParsePublicKey(mustBase64(fields[2])); err == nil {
			entry.SHA256Fingerprint = ssh.FingerprintSHA256(key)
		}
	}
	if len(fields) >= 4 {
		entry.Comment = strings.Join(fields[3:], " ")
	}
	return entry, true
}

func (v *HostKeyVerifier) DeleteManagedLine(line int) error {
	if v == nil || v.managedPath == "" {
		return ErrInvalidKnownHostsPath
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	data, err := os.ReadFile(v.managedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.ErrNotExist
		}
		return fmt.Errorf("read managed known_hosts: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return fmt.Errorf("known_hosts line %d is out of range", line)
	}
	target := strings.TrimSpace(lines[line-1])
	if target == "" || strings.HasPrefix(target, "#") {

		return fmt.Errorf("known_hosts line %d is not a host key entry", line)
	}
	remaining := append(append([]string{}, lines[:line-1]...), lines[line:]...)
	output := strings.Join(remaining, "\n")

	info, statErr := os.Stat(v.managedPath)
	mode := fs.FileMode(0o600)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(v.managedPath), ".known_hosts-*")
	if err != nil {
		return fmt.Errorf("create known_hosts temporary: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryPath) }
	if _, err := temporary.WriteString(output); err != nil {
		_ = temporary.Close()
		cleanup()
		return fmt.Errorf("write known_hosts temporary: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		cleanup()
		return fmt.Errorf("sync known_hosts temporary: %w", err)
	}
	if err := temporary.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close known_hosts temporary: %w", err)
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod known_hosts temporary: %w", err)
	}
	if err := os.Rename(temporaryPath, v.managedPath); err != nil {
		cleanup()
		return fmt.Errorf("replace known_hosts: %w", err)
	}
	return nil
}

func mustBase64(value string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil
	}
	return decoded
}
