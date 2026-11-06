package hostimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type electermGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	BookmarkIDs      []string `json:"bookmarkIds"`
	BookmarkGroupIDs []string `json:"bookmarkGroupIds"`
}

type electermBookmark struct {
	ID string `json:"id"`

	Title string `json:"title"`
	Host  string `json:"host"`

	Username string `json:"username"`
	AuthType string `json:"authType"`

	Port *int64 `json:"port"`
	Type string `json:"type"`

	EnableSSH *bool `json:"enableSsh"`
}

type electermFile struct {
	BookmarkGroups []electermGroup    `json:"bookmarkGroups"`
	Bookmarks      []electermBookmark `json:"bookmarks"`
}

func DefaultElectermPath() string {
	return ""
}

func ParseElecterm(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultElectermPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read electerm bookmarks: %w", err)
	}
	return ParseElectermBytes(raw)
}

func ParseElectermBytes(raw []byte) (Result, error) {
	var file electermFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return Result{}, fmt.Errorf("parse electerm bookmarks: %w", err)
	}
	groups := make(map[string]electermGroup, len(file.BookmarkGroups))
	for _, group := range file.BookmarkGroups {

		if _, exists := groups[group.ID]; !exists {
			groups[group.ID] = group
		}
	}

	bookmarkGroup := make(map[string]string, len(file.Bookmarks))
	for _, group := range file.BookmarkGroups {
		for _, bookmarkID := range group.BookmarkIDs {
			if _, exists := bookmarkGroup[bookmarkID]; !exists {
				bookmarkGroup[bookmarkID] = group.ID
			}
		}
	}
	childParent := make(map[string]string, len(file.BookmarkGroups))
	for _, group := range file.BookmarkGroups {
		for _, childID := range group.BookmarkGroupIDs {
			if _, exists := childParent[childID]; !exists {
				childParent[childID] = group.ID
			}
		}
	}

	result := Result{Entries: make([]Entry, 0, len(file.Bookmarks))}
	for index, bookmark := range file.Bookmarks {
		if !strings.EqualFold(strings.TrimSpace(bookmark.Type), "ssh") {
			continue
		}
		if bookmark.EnableSSH != nil && !*bookmark.EnableSSH {
			continue
		}
		host := strings.TrimSpace(bookmark.Host)
		if host == "" {
			continue
		}
		port := 22
		if bookmark.Port != nil {
			if *bookmark.Port <= 0 || *bookmark.Port > 65535 {
				result.Skipped++
				continue
			}
			port = int(*bookmark.Port)
		}
		name := strings.TrimSpace(bookmark.Title)
		if name == "" {
			name = host
		}
		username := strings.TrimSpace(bookmark.Username)
		if username == "" {
			username = "root"
		}
		group := ""
		if groupID, ok := bookmarkGroup[bookmark.ID]; ok {
			group = electermGroupPath(groupID, groups, childParent)
		}
		result.Entries = append(result.Entries, Entry{
			Alias:    name,
			HostName: host,
			User:     username,
			Port:     port,
			Group:    group,
			Line:     index + 1,
		})
	}
	return result, nil
}

func electermGroupPath(groupID string, groups map[string]electermGroup, childParent map[string]string) string {
	segments := make([]string, 0, 4)
	visited := make(map[string]struct{}, 4)
	current := groupID
	for {
		if _, seen := visited[current]; seen {
			break
		}
		visited[current] = struct{}{}
		group, ok := groups[current]
		if !ok {

			return ""
		}
		if title := strings.TrimSpace(group.Title); title != "" {
			segments = append(segments, title)
		}
		parentID, ok := childParent[current]
		if !ok {
			break
		}
		current = parentID
	}
	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}
	return joinGroupSegments(segments)
}
