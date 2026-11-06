package hostimport

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ParseSecureCRTSessions(root string) (Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("stat securecrt sessions: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("securecrt sessions path is not a directory")
	}
	result := Result{Entries: make([]Entry, 0, 16)}
	line := 0
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".ini") {
			return nil
		}
		line++
		parsed, ok := parseSecureCRTSessionFile(path)
		if !ok {
			result.Skipped++
			return nil
		}
		parsed.Line = line
		result.Entries = append(result.Entries, parsed)
		return nil
	})
	if walkErr != nil {
		return Result{}, fmt.Errorf("walk securecrt sessions: %w", walkErr)
	}
	return result, nil
}

func DefaultSecureCRTSessionsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "VanDyke", "SecureCRT", "Config", "Sessions")
}

func parseSecureCRTSessionFile(path string) (Entry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, false
	}
	entry := Entry{
		Alias: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Port:  22,
	}
	protocol := ""
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		name, value, ok := splitSecureCRTValue(line)
		if !ok {
			continue
		}
		switch strings.ToLower(name) {
		case "protocol name":
			protocol = strings.ToUpper(value)
		case "hostname":
			entry.HostName = value
		case "username":
			entry.User = value
		case "[ssh2] port", "[ssh1] port", "port":
			if port, err := strconv.ParseInt(value, 16, 32); err == nil && port > 0 && port <= 65535 {
				entry.Port = int(port)
			}
		}
	}
	if protocol != "" && protocol != "SSH2" && protocol != "SSH1" {
		return Entry{}, false
	}
	if entry.HostName == "" {
		return Entry{}, false
	}
	return entry, true
}

func splitSecureCRTValue(line string) (string, string, bool) {
	equals := strings.Index(line, "=")
	if equals <= 0 {
		return "", "", false
	}
	left := strings.TrimSpace(line[:equals])
	value := strings.TrimSpace(line[equals+1:])

	if index := strings.Index(left, `"`); index >= 0 {
		left = strings.Trim(left[index:], `"`)
	}
	return left, strings.Trim(value, `"`), left != ""
}
