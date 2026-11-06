package hostimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func DefaultFinalShellPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".finalshell", "conn")
}

type finalShellFolder struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ParentID   *string `json:"parent_id"`
	DeleteTime uint64  `json:"delete_time"`
}

type finalShellConnection struct {
	Name string `json:"name"`
	Host string `json:"host"`

	Port *uint64 `json:"port"`

	UserName *string `json:"user_name"`
	ParentID *string `json:"parent_id"`

	ConnectionType *int   `json:"conection_type"`
	DeleteTime     uint64 `json:"delete_time"`
}

func ParseFinalShell(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultFinalShellPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("stat finalshell conn directory: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("finalshell conn path is not a directory")
	}

	folders := make(map[string]finalShellFolder, 16)
	connections := make([]finalShellConnection, 0, 32)

	walkErr := filepath.WalkDir(path, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch {
		case entry.Name() == "folder.json":
			var folder finalShellFolder
			if err := decodeFinalShellJSON(entryPath, &folder); err != nil {
				return err
			}
			if folder.DeleteTime == 0 {
				folders[folder.ID] = folder
			}
		case strings.HasSuffix(entry.Name(), "_connect_config.json"):
			var connection finalShellConnection
			if err := decodeFinalShellJSON(entryPath, &connection); err != nil {
				return err
			}
			connections = append(connections, connection)
		}
		return nil
	})
	if walkErr != nil {
		return Result{}, fmt.Errorf("walk finalshell conn directory: %w", walkErr)
	}
	if len(connections) == 0 {
		return Result{}, fmt.Errorf("finalshell conn directory has no *_connect_config.json files")
	}

	result := Result{Entries: make([]Entry, 0, len(connections))}
	for index, connection := range connections {
		entry, ok := finalShellEntry(connection, folders)
		if !ok {
			continue
		}
		entry.Line = index + 1
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func finalShellEntry(connection finalShellConnection, folders map[string]finalShellFolder) (Entry, bool) {
	if connection.DeleteTime != 0 || connection.ConnectionType == nil || *connection.ConnectionType != 100 {
		return Entry{}, false
	}
	host := strings.TrimSpace(connection.Host)
	if host == "" {
		return Entry{}, false
	}
	port := 22
	if connection.Port != nil && *connection.Port > 0 && *connection.Port <= 65535 {
		port = int(*connection.Port)
	}
	username := "root"
	if connection.UserName != nil {
		if value := strings.TrimSpace(*connection.UserName); value != "" {
			username = value
		}
	}
	name := strings.TrimSpace(connection.Name)
	if name == "" {
		name = host
	}
	group := ""
	var parentID string
	if connection.ParentID != nil {
		parentID = *connection.ParentID
	}
	group = finalShellGroupPath(parentID, folders)
	return Entry{
		Alias:    name,
		HostName: host,
		User:     username,
		Port:     port,
		Group:    group,
	}, true
}

func finalShellGroupPath(parentID string, folders map[string]finalShellFolder) string {
	if strings.TrimSpace(parentID) == "" || parentID == "root" || parentID == "0" {
		return ""
	}
	segments := make([]string, 0, 4)
	current := parentID
	for range folders {
		folder, ok := folders[current]
		if !ok {
			return ""
		}
		segments = append(segments, folder.Name)
		if folder.ParentID == nil {
			break
		}
		next := *folder.ParentID
		if next == "root" || next == "0" || strings.TrimSpace(next) == "" {
			break
		}
		current = next
	}

	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}
	return joinGroupSegments(segments)
}

func decodeFinalShellJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read finalshell json: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse finalshell json %s: %w", filepath.Base(path), err)
	}
	return nil
}
