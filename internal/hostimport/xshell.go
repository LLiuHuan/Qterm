package hostimport

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	xshellMaxArchiveEntries = 10_000

	xshellMaxExpandedBytes = 50 << 20
)

func DefaultXshellPath() string {
	return ""
}

func ParseXshell(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultXshellPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("open xshell archive: %w", err)
	}
	defer archive.Close()
	return parseXshellArchive(archive)
}

func parseXshellArchive(archive *zip.ReadCloser) (Result, error) {
	if len(archive.File) > xshellMaxArchiveEntries {
		return Result{}, fmt.Errorf("xshell archive has too many entries")
	}
	result := Result{Entries: make([]Entry, 0, 16)}
	var expanded uint64
	for index, file := range archive.File {

		entryPath := decodeLegacyText([]byte(file.Name))
		if !strings.EqualFold(path.Ext(entryPath), ".xsh") {
			continue
		}
		expanded += file.UncompressedSize64
		if expanded > xshellMaxExpandedBytes {
			return Result{}, fmt.Errorf("xshell archive exceeds size limit")
		}
		reader, openErr := file.Open()
		if openErr != nil {
			return Result{}, fmt.Errorf("open xshell archive entry: %w", openErr)
		}
		raw, readErr := io.ReadAll(io.LimitReader(reader, xshellMaxExpandedBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return Result{}, fmt.Errorf("read xshell archive entry: %w", readErr)
		}
		if closeErr != nil {
			return Result{}, fmt.Errorf("close xshell archive entry: %w", closeErr)
		}
		if len(raw) > xshellMaxExpandedBytes {
			return Result{}, fmt.Errorf("xshell archive exceeds size limit")
		}
		entry, ok := parseXshellSession(decodeLegacyText(raw), entryPath)
		if !ok {
			result.Skipped++
			continue
		}
		entry.Line = index + 1
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func parseXshellSession(content string, entryPath string) (Entry, bool) {
	document := parseINISections(content)
	connection := document.section("CONNECTION")
	if connection == nil {
		return Entry{}, false
	}
	if !strings.EqualFold(connection.value("Protocol"), "SSH") {
		return Entry{}, false
	}
	host := strings.TrimSpace(connection.value("Host"))
	if host == "" {
		return Entry{}, false
	}

	port := 22
	if parsed, err := strconv.Atoi(connection.value("Port")); err == nil && parsed > 0 && parsed <= 65535 {
		port = parsed
	}
	username := "root"
	if authentication := document.section("CONNECTION:AUTHENTICATION"); authentication != nil {
		if value := strings.TrimSpace(authentication.value("UserName")); value != "" {
			username = value
		}
	}
	base := path.Base(entryPath)
	return Entry{
		Alias:    strings.TrimSuffix(base, path.Ext(base)),
		HostName: host,
		User:     username,
		Port:     port,
		Group:    xshellGroupPath(entryPath),
	}, true
}

func xshellGroupPath(entryPath string) string {
	directory := path.Dir(entryPath)
	switch {
	case directory == "." || directory == "/" || directory == "Xshell" || directory == "Xshell/Sessions":
		return ""
	case strings.HasPrefix(directory, "Xshell/Sessions/"):
		directory = strings.TrimPrefix(directory, "Xshell/Sessions/")
	case strings.HasPrefix(directory, "Xshell/"):
		directory = strings.TrimPrefix(directory, "Xshell/")
	}
	return joinGroupSegments(strings.Split(directory, "/"))
}
