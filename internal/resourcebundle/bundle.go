package resourcebundle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/security"
)

const (
	Magic          = "QTRB"
	Version        = byte(1)
	manifestName   = "manifest.json"
	archiveAAD     = "qterm/resource-bundle/v1"
	maxBundleBytes = int64(1 << 30)
	maxEntryBytes  = int64(1 << 30)
)

type Manifest struct {
	Format     string            `json:"format"`
	Scope      string            `json:"scope"`
	Version    int               `json:"version"`
	CreatedAt  time.Time         `json:"createdAt"`
	Files      map[string]string `json:"files"`
	Recordings map[string]string `json:"recordings,omitempty"`
}

type Entry struct {
	Name string
	Path string
	Data []byte
}

func Write(ctx context.Context, path string, password []byte, manifest Manifest, entries []Entry) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateBundlePath(path); err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("resource bundle is empty")
	}
	manifest.Format = "Qterm resource bundle"
	if strings.TrimSpace(manifest.Scope) == "" {
		manifest.Scope = "assets"
	}
	manifest.Version = int(Version)
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	}
	manifest.Files = make(map[string]string, len(entries))
	manifest.Recordings = cloneStringMap(manifest.Recordings)

	var archive bytes.Buffer
	archiveWriter := zip.NewWriter(&archive)
	seenEntries := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if err := validateEntry(entry); err != nil {
			_ = archiveWriter.Close()
			return err
		}
		if _, exists := seenEntries[entry.Name]; exists {
			_ = archiveWriter.Close()
			return fmt.Errorf("duplicate resource entry %q", entry.Name)
		}
		seenEntries[entry.Name] = struct{}{}
		if err := validateContext(ctx); err != nil {
			_ = archiveWriter.Close()
			return err
		}
		hash := sha256.New()
		writer, err := archiveWriter.Create(entry.Name)
		if err != nil {
			_ = archiveWriter.Close()
			return fmt.Errorf("create resource entry %q: %w", entry.Name, err)
		}
		var source io.Reader
		var file *os.File
		if entry.Path != "" {
			file, err = os.Open(entry.Path)
			if err != nil {
				_ = archiveWriter.Close()
				return fmt.Errorf("open resource entry %q: %w", entry.Name, err)
			}
			info, statErr := file.Stat()
			if statErr != nil {
				_ = file.Close()
				_ = archiveWriter.Close()
				return fmt.Errorf("stat resource entry %q: %w", entry.Name, statErr)
			}
			if !info.Mode().IsRegular() || info.Size() > maxEntryBytes {
				_ = file.Close()
				_ = archiveWriter.Close()
				return fmt.Errorf("resource entry %q is invalid or too large", entry.Name)
			}
			source = file
		} else {
			if int64(len(entry.Data)) > maxEntryBytes {
				_ = archiveWriter.Close()
				return fmt.Errorf("resource entry %q is too large", entry.Name)
			}
			source = bytes.NewReader(entry.Data)
		}
		tee := io.MultiWriter(writer, hash)
		if _, err := io.Copy(tee, source); err != nil {
			if file != nil {
				_ = file.Close()
			}
			_ = archiveWriter.Close()
			return fmt.Errorf("read resource entry %q: %w", entry.Name, err)
		}
		if file != nil {
			if err := file.Close(); err != nil {
				_ = archiveWriter.Close()
				return fmt.Errorf("close resource entry %q: %w", entry.Name, err)
			}
		}
		manifest.Files[entry.Name] = hex.EncodeToString(hash.Sum(nil))
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		_ = archiveWriter.Close()
		return fmt.Errorf("encode resource manifest: %w", err)
	}
	manifestWriter, err := archiveWriter.Create(manifestName)
	if err != nil {
		_ = archiveWriter.Close()
		return fmt.Errorf("create resource manifest: %w", err)
	}
	if _, err := manifestWriter.Write(manifestBytes); err != nil {
		_ = archiveWriter.Close()
		return fmt.Errorf("write resource manifest: %w", err)
	}
	if err := archiveWriter.Close(); err != nil {
		return fmt.Errorf("close resource archive: %w", err)
	}
	if int64(archive.Len()) > maxBundleBytes {
		return errors.New("resource bundle is too large")
	}
	return writeEncrypted(ctx, path, password, archive.Bytes())
}

func Extract(ctx context.Context, path string, password []byte, destination string) (Manifest, error) {
	if err := validateContext(ctx); err != nil {
		return Manifest{}, err
	}
	if err := validateBundlePath(path); err != nil {
		return Manifest{}, err
	}
	if destination == "" || !filepath.IsAbs(destination) {
		return Manifest{}, errors.New("resource extraction path must be absolute")
	}
	manifest, entries, err := ExtractData(ctx, path, password)
	if err != nil {
		return Manifest{}, err
	}
	defer clearEntryMap(entries)
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create resource extraction directory: %w", err)
	}
	for name, data := range entries {
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := validateExtractedPath(destination, target); err != nil {
			return Manifest{}, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Manifest{}, fmt.Errorf("create resource entry directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return Manifest{}, fmt.Errorf("write resource entry %q: %w", name, err)
		}
	}
	return manifest, nil
}

func ExtractData(ctx context.Context, path string, password []byte) (Manifest, map[string][]byte, error) {
	if err := validateContext(ctx); err != nil {
		return Manifest{}, nil, err
	}
	if err := validateBundlePath(path); err != nil {
		return Manifest{}, nil, err
	}
	archiveBytes, err := readEncrypted(ctx, path, password)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer clear(archiveBytes)
	return extractDataBytes(ctx, archiveBytes)
}

func ExtractDataBytes(ctx context.Context, data, password []byte) (Manifest, map[string][]byte, error) {
	if err := validateContext(ctx); err != nil {
		return Manifest{}, nil, err
	}
	if len(data) == 0 {
		return Manifest{}, nil, errors.New("resource package is empty")
	}
	archiveBytes, err := decryptArchiveBytes(data, password)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer clear(archiveBytes)
	return extractDataBytes(ctx, archiveBytes)
}

func extractDataBytes(ctx context.Context, archiveBytes []byte) (Manifest, map[string][]byte, error) {
	if err := validateContext(ctx); err != nil {
		return Manifest{}, nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("open resource archive: %w", err)
	}
	manifest, err := readManifest(reader)
	if err != nil {
		return Manifest{}, nil, err
	}
	entries := make(map[string][]byte, len(manifest.Files))
	for name, expectedHash := range manifest.Files {
		if err := validateEntry(Entry{Name: name}); err != nil {
			clearEntryMap(entries)
			return Manifest{}, nil, err
		}
		file := findFile(reader.File, name)
		if file == nil {
			clearEntryMap(entries)
			return Manifest{}, nil, fmt.Errorf("resource entry %q is missing", name)
		}
		data, err := readZipFile(file)
		if err != nil {
			clearEntryMap(entries)
			return Manifest{}, nil, fmt.Errorf("read resource entry %q: %w", name, err)
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(expectedHash, hex.EncodeToString(hash[:])) {
			clear(data)
			clearEntryMap(entries)
			return Manifest{}, nil, fmt.Errorf("resource entry %q checksum mismatch", name)
		}
		entries[name] = data
	}
	return manifest, entries, nil
}

func readManifest(reader *zip.Reader) (Manifest, error) {
	manifestFile := findFile(reader.File, manifestName)
	if manifestFile == nil {
		return Manifest{}, errors.New("resource manifest is missing")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return Manifest{}, fmt.Errorf("read resource manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode resource manifest: %w", err)
	}
	if manifest.Format != "Qterm resource bundle" || manifest.Scope != "assets" || manifest.Version != int(Version) || len(manifest.Files) == 0 {
		return Manifest{}, errors.New("unsupported resource bundle manifest")
	}
	return manifest, nil
}

func clearEntryMap(entries map[string][]byte) {
	for name, data := range entries {
		clear(data)
		delete(entries, name)
	}
}

func writeEncrypted(ctx context.Context, path string, password, plaintext []byte) error {
	archiveKey, err := security.GenerateDEK()
	if err != nil {
		return err
	}
	defer clear(archiveKey)
	wrappedKey, err := security.CreateRecoveryPackage(archiveKey, password)
	if err != nil {
		return err
	}
	defer clear(wrappedKey)
	block, err := aes.NewCipher(archiveKey)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	defer clear(nonce)
	sealed := aead.Seal(nil, nonce, plaintext, []byte(archiveAAD))
	output := make([]byte, 0, len(Magic)+1+4+len(wrappedKey)+len(nonce)+len(sealed))
	output = append(output, Magic...)
	output = append(output, Version)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(wrappedKey)))
	output = append(output, length[:]...)
	output = append(output, wrappedKey...)
	output = append(output, nonce...)
	output = append(output, sealed...)
	if err := validateContext(ctx); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".qterm-resource-*")
	if err != nil {
		return fmt.Errorf("create resource package temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("restrict resource package permissions: %w", err)
	}
	if _, err := tmp.Write(output); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write resource package: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close resource package: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace resource package: %w", err)
	}
	return nil
}

func readEncrypted(ctx context.Context, path string, password []byte) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat resource package: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxBundleBytes {
		return nil, errors.New("resource package is invalid or too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resource package: %w", err)
	}
	return decryptArchiveBytes(data, password)
}

func decryptArchiveBytes(data, password []byte) ([]byte, error) {
	if len(data) < len(Magic)+1+4 || string(data[:len(Magic)]) != Magic || data[len(Magic)] != Version {
		return nil, errors.New("invalid resource package header")
	}
	offset := len(Magic) + 1
	wrappedLength := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	if wrappedLength <= 0 || wrappedLength > len(data)-offset {
		return nil, errors.New("invalid resource package key envelope")
	}
	wrappedKey := data[offset : offset+wrappedLength]
	offset += wrappedLength
	archiveKey, err := security.OpenRecoveryPackage(wrappedKey, password)
	if err != nil {
		return nil, err
	}
	defer clear(archiveKey)
	block, err := aes.NewCipher(archiveKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data)-offset < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("resource package payload is truncated")
	}
	nonce := data[offset : offset+aead.NonceSize()]
	plaintext, err := aead.Open(nil, nonce, data[offset+aead.NonceSize():], []byte(archiveAAD))
	if err != nil {
		return nil, errors.New("resource package authentication failed")
	}
	return plaintext, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	if file == nil || file.FileInfo().IsDir() || file.UncompressedSize64 > uint64(maxEntryBytes) {
		return nil, errors.New("invalid resource archive entry")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxEntryBytes {
		return nil, errors.New("resource archive entry is too large")
	}
	return data, nil
}

func findFile(files []*zip.File, name string) *zip.File {
	for _, file := range files {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func validateEntry(entry Entry) error {
	if entry.Name == "" || entry.Name == manifestName || filepath.IsAbs(entry.Name) {
		return errors.New("invalid resource entry name")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry.Name)))
	if clean == "." || clean != entry.Name || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return errors.New("resource entry escapes package root")
	}
	if entry.Path != "" {
		if !filepath.IsAbs(entry.Path) {
			return errors.New("resource entry path must be absolute")
		}
	}
	return nil
}

func validateExtractedPath(root, target string) error {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cleanTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("resource entry escapes extraction directory")
	}
	return nil
}

func validateBundlePath(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) == filepath.VolumeName(path)+string(os.PathSeparator) {
		return errors.New("resource package path is invalid")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create resource package directory: %w", err)
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("resource context is nil")
	}
	return ctx.Err()
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
