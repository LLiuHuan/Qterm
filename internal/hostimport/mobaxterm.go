package hostimport

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func DefaultMobaXtermPath() string {
	return ""
}

func ParseMobaXterm(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultMobaXtermPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read mobaxterm sessions: %w", err)
	}
	return ParseMobaXtermBytes(raw)
}

func ParseMobaXtermBytes(raw []byte) (Result, error) {
	document := parseINISections(decodeLegacyText(raw))
	result := Result{Entries: make([]Entry, 0, 16)}
	line := 0
	for _, section := range document.sections {
		if !strings.HasPrefix(section.name, "Bookmarks") {
			continue
		}
		group := joinGroupSegments(strings.Split(section.value("SubRep"), `\`))
		for _, key := range section.keys {
			if key == "SubRep" || key == "ImgNum" {
				continue
			}
			line++
			entry, ok := parseMobaXtermEntry(key, section.value(key), group)
			if !ok {
				continue
			}
			entry.Line = line
			result.Entries = append(result.Entries, entry)
		}
	}
	return result, nil
}

func parseMobaXtermEntry(name string, value string, group string) (Entry, bool) {
	hashIndex := strings.Index(value, "#")
	if hashIndex < 0 {
		return Entry{}, false
	}
	afterHash := value[hashIndex+1:]
	percentIndex := strings.Index(afterHash, "%")
	if percentIndex < 0 {
		return Entry{}, false
	}

	if !strings.HasPrefix(afterHash[:percentIndex], "109") {
		return Entry{}, false
	}
	fields := strings.Split(afterHash[percentIndex+1:], "%")
	if len(fields) < 3 {
		return Entry{}, false
	}
	host := strings.TrimSpace(fields[0])
	if host == "" {
		return Entry{}, false
	}
	port := 22
	if parsed, err := strconv.Atoi(strings.TrimSpace(fields[1])); err == nil && parsed > 0 && parsed <= 65535 {
		port = parsed
	}
	username := strings.TrimSpace(fields[2])
	if username == "" {
		username = "root"
	}
	return Entry{
		Alias:    strings.TrimSpace(name),
		HostName: host,
		User:     username,
		Port:     port,
		Group:    group,
	}, true
}
