package hostimport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ParseTermiusJSON(data []byte) (Result, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Result{}, fmt.Errorf("decode termius export: %w", err)
	}
	hosts := findTermiusHosts(raw)
	result := Result{Entries: make([]Entry, 0, len(hosts))}
	for index, item := range hosts {
		entry, ok := termiusEntry(item, index+1)
		if !ok {
			result.Skipped++
			continue
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func ParseTermiusFile(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read termius export: %w", err)
	}
	return ParseTermiusJSON(data)
}

func DefaultTermiusPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "termius.json")
}

func findTermiusHosts(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case map[string]any:
		if hosts, ok := typed["hosts"].([]any); ok {
			return hosts
		}
		for _, nested := range typed {
			if hosts := findTermiusHosts(nested); len(hosts) > 0 {
				return hosts
			}
		}
	}
	return nil
}

func termiusEntry(value any, index int) (Entry, bool) {
	item, ok := value.(map[string]any)
	if !ok {
		return Entry{}, false
	}
	address := firstString(item, "address", "hostname", "host")
	if address == "" {
		return Entry{}, false
	}
	alias := firstString(item, "label", "name", "title")
	if alias == "" {
		alias = address
	}
	entry := Entry{
		Alias:    strings.TrimSpace(alias),
		HostName: strings.TrimSpace(address),
		User:     strings.TrimSpace(firstString(item, "username", "user")),
		Port:     firstInt(item, "port"),
		Line:     index,
	}
	if entry.Port == 0 {
		entry.Port = 22
	}
	return entry, true
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := item[key]; ok {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		switch raw := item[key].(type) {
		case float64:
			return int(raw)
		case string:
			if parsed, ok := parseInt(raw); ok {
				return parsed
			}
		}
	}
	return 0
}

func parseInt(text string) (int, bool) {
	value := 0
	for _, char := range strings.TrimSpace(text) {
		if char < '0' || char > '9' {
			return 0, false
		}
		value = value*10 + int(char-'0')
	}
	if value == 0 {
		return 0, false
	}
	return value, true
}
