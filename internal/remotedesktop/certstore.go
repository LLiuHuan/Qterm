package remotedesktop

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	certificateStoreVersion = 1

	certificateStoreFileName = "trusted-certificates.json"

	defaultRDPPort = 3389
)

var (
	ErrCertificateFingerprintInvalid = errors.New("证书指纹格式非法")

	ErrCertificateStoreUnavailable = errors.New("证书信任库不可用")

	ErrCertificateStoreCorrupt = errors.New("证书信任库文件已损坏")
)

type TrustedCertificate struct {
	Fingerprint string `json:"fingerprint"`

	Subject string `json:"subject,omitempty"`
	Issuer  string `json:"issuer,omitempty"`

	TrustedAt string `json:"trustedAt,omitempty"`
}

type certificateStoreFile struct {
	Version      int                           `json:"version"`
	Certificates map[string]TrustedCertificate `json:"certificates"`
}

type CertificateStore struct {
	mu   sync.Mutex
	root string
	path string

	entries map[string]TrustedCertificate
	loaded  bool

	loadErr error
}

func NewCertificateStore(root string) *CertificateStore {
	trimmed := strings.TrimSpace(root)
	store := &CertificateStore{root: trimmed}
	if trimmed != "" {
		store.path = filepath.Join(trimmed, "rdp", certificateStoreFileName)
	}
	return store
}

func (s *CertificateStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *CertificateStore) Lookup(host string, port int) (TrustedCertificate, bool) {
	record, found, _ := s.LookupChecked(host, port)
	return record, found
}

func (s *CertificateStore) LookupChecked(host string, port int) (TrustedCertificate, bool, error) {
	if s == nil || s.path == "" {
		return TrustedCertificate{}, false, ErrCertificateStoreUnavailable
	}
	key := certificateStoreKey(host, port)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return TrustedCertificate{}, false, err
	}
	record, found := s.entries[key]
	return record, found, nil
}

func (s *CertificateStore) Trust(host string, port int, fingerprint, subject, issuer string) error {
	if s == nil || s.path == "" {
		return ErrCertificateStoreUnavailable
	}
	normalized, err := normalizeCertificateFingerprintChecked(fingerprint)
	if err != nil {
		return err
	}
	key := certificateStoreKey(host, port)
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.loadLocked()
	if s.entries == nil {
		s.entries = map[string]TrustedCertificate{}
	}
	previous, hadPrevious := s.entries[key]
	s.entries[key] = TrustedCertificate{
		Fingerprint: normalized,
		Subject:     strings.TrimSpace(subject),
		Issuer:      strings.TrimSpace(issuer),
		TrustedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.saveLocked(); err != nil {

		if hadPrevious {
			s.entries[key] = previous
		} else {
			delete(s.entries, key)
		}
		return err
	}
	return nil
}

func (s *CertificateStore) Forget(host string, port int) error {
	if s == nil || s.path == "" {
		return ErrCertificateStoreUnavailable
	}
	key := certificateStoreKey(host, port)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return err
	}
	previous, found := s.entries[key]
	if !found {
		return nil
	}
	delete(s.entries, key)
	if err := s.saveLocked(); err != nil {
		s.entries[key] = previous
		return err
	}
	return nil
}

func (s *CertificateStore) Entries() (map[string]TrustedCertificate, error) {
	if s == nil || s.path == "" {
		return nil, ErrCertificateStoreUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]TrustedCertificate, len(s.entries))
	for _, key := range keys {
		result[key] = s.entries[key]
	}
	return result, nil
}

func (s *CertificateStore) loadLocked() error {
	if s.loaded {
		return s.loadErr
	}
	s.loaded = true
	s.entries = map[string]TrustedCertificate{}
	s.loadErr = nil
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		s.loadErr = fmt.Errorf("%w: %v", ErrCertificateStoreUnavailable, err)
		return s.loadErr
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var file certificateStoreFile
	if err := json.Unmarshal(raw, &file); err != nil {

		s.entries = map[string]TrustedCertificate{}
		s.loadErr = fmt.Errorf("%w: %v", ErrCertificateStoreCorrupt, err)
		return s.loadErr
	}
	if file.Version != 0 && file.Version != certificateStoreVersion {
		s.entries = map[string]TrustedCertificate{}
		s.loadErr = fmt.Errorf("%w: 未知的信任库版本 %d", ErrCertificateStoreCorrupt, file.Version)
		return s.loadErr
	}
	for key, record := range file.Certificates {
		normalizedKey := normalizeCertificateStoreKey(key)
		if normalizedKey == "" {
			continue
		}
		normalizedFingerprint, err := normalizeCertificateFingerprintChecked(record.Fingerprint)
		if err != nil {

			continue
		}
		record.Fingerprint = normalizedFingerprint
		s.entries[normalizedKey] = record
	}
	return nil
}

func (s *CertificateStore) saveLocked() error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("%w: 创建信任库目录失败: %v", ErrCertificateStoreUnavailable, err)
	}
	payload, err := json.MarshalIndent(certificateStoreFile{
		Version:      certificateStoreVersion,
		Certificates: s.entries,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: 编码信任库失败: %v", ErrCertificateStoreUnavailable, err)
	}
	payload = append(payload, '\n')
	temporary, err := os.CreateTemp(directory, ".trusted-certificates-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: 创建临时文件失败: %v", ErrCertificateStoreUnavailable, err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}

	if err := temporary.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("%w: 设置信任库权限失败: %v", ErrCertificateStoreUnavailable, err)
	}
	if _, err := temporary.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("%w: 写入信任库失败: %v", ErrCertificateStoreUnavailable, err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("%w: 同步信任库失败: %v", ErrCertificateStoreUnavailable, err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("%w: 关闭临时文件失败: %v", ErrCertificateStoreUnavailable, err)
	}

	if err := os.Rename(temporaryPath, s.path); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("%w: 替换信任库失败: %v", ErrCertificateStoreUnavailable, err)
	}

	s.loadErr = nil

	if handle, err := os.Open(directory); err == nil {
		_ = handle.Sync()
		_ = handle.Close()
	}
	return nil
}

func certificateStoreKey(host string, port int) string {
	return net.JoinHostPort(normalizeCertificateHost(host), strconv.Itoa(normalizeCertificatePort(port)))
}

func normalizeCertificateStoreKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	host, portText, err := net.SplitHostPort(trimmed)
	if err != nil {

		return certificateStoreKey(trimmed, defaultRDPPort)
	}
	port, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil {
		return certificateStoreKey(host, defaultRDPPort)
	}
	return certificateStoreKey(host, port)
}

func normalizeCertificateHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func normalizeCertificatePort(port int) int {
	if port <= 0 || port > 65535 {
		return defaultRDPPort
	}
	return port
}

func NormalizeCertificateFingerprint(value string) string {
	normalized, err := normalizeCertificateFingerprintChecked(value)
	if err != nil {
		return ""
	}
	return normalized
}

func normalizeCertificateFingerprintChecked(value string) (string, error) {
	replaced := strings.NewReplacer(":", "", " ", "", "-", "", "\t", "").Replace(value)
	trimmed := strings.ToLower(strings.TrimSpace(replaced))
	if trimmed == "" {
		return "", fmt.Errorf("%w: 指纹为空", ErrCertificateFingerprintInvalid)
	}
	if len(trimmed) != 64 {
		return "", fmt.Errorf("%w: 期望 64 位十六进制，实际 %d 位", ErrCertificateFingerprintInvalid, len(trimmed))
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return "", fmt.Errorf("%w: %v", ErrCertificateFingerprintInvalid, err)
	}
	return trimmed, nil
}
