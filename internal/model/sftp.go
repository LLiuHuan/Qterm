package model

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxRemotePathSize = 4096
	maxRemoteNameSize = 1024
)

var ErrInvalidRemotePath = errors.New("invalid remote path")

type RemoteFileType string

const (
	RemoteFileTypeFile      RemoteFileType = "file"
	RemoteFileTypeDirectory RemoteFileType = "directory"
	RemoteFileTypeSymlink   RemoteFileType = "symlink"
	RemoteFileTypeOther     RemoteFileType = "other"
)

type RemoteFileEntry struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Directory string         `json:"directory"`
	Type      RemoteFileType `json:"type"`

	TargetType RemoteFileType `json:"targetType,omitempty"`
	Size       int64          `json:"size"`
	SizeText   string         `json:"sizeText"`
	Mode       string         `json:"mode"`
	UID        uint32         `json:"uid"`
	GID        uint32         `json:"gid"`
	ModTime    time.Time      `json:"modTime"`
}

type RemotePrincipal struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

type RemotePrincipals struct {
	Users  []RemotePrincipal `json:"users"`
	Groups []RemotePrincipal `json:"groups"`
}

func NormalizeRemotePath(value string) (string, error) {
	if value == "" || len(value) > maxRemotePathSize || !utf8.ValidString(value) ||
		strings.IndexByte(value, 0) >= 0 || !strings.HasPrefix(value, "/") {
		return "", ErrInvalidRemotePath
	}
	return path.Clean(value), nil
}

func NormalizeRemoteDirectory(value string) (string, error) {
	return NormalizeRemotePath(value)
}

func RemoteFileTypeFromMode(mode fs.FileMode) RemoteFileType {
	switch {
	case mode&fs.ModeSymlink != 0:
		return RemoteFileTypeSymlink
	case mode.IsDir():
		return RemoteFileTypeDirectory
	case !mode.IsRegular():
		return RemoteFileTypeOther
	default:
		return RemoteFileTypeFile
	}
}

func NewRemoteFileEntry(directory string, info fs.FileInfo) (RemoteFileEntry, error) {
	directory, err := NormalizeRemoteDirectory(directory)
	if err != nil || info == nil {
		return RemoteFileEntry{}, ErrInvalidRemotePath
	}
	name := info.Name()
	if name == "" || name == "." || name == ".." || len(name) > maxRemoteNameSize ||
		!utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 || path.Base(name) != name {
		return RemoteFileEntry{}, ErrInvalidRemotePath
	}
	entryType := RemoteFileTypeFromMode(info.Mode())
	return RemoteFileEntry{
		Name:      name,
		Path:      path.Join(directory, name),
		Directory: directory,
		Type:      entryType,
		Size:      info.Size(),
		SizeText:  FormatBytes(uint64(nonNegativeSize(info.Size()))),
		Mode:      info.Mode().String(),
		ModTime:   info.ModTime().UTC(),
	}, nil
}

func nonNegativeSize(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
