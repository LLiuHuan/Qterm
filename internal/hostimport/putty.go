package hostimport

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

func ParsePuttyReg(data []byte) (Result, error) {
	text := decodeRegText(data)
	result := Result{Entries: make([]Entry, 0, 8)}
	current := (*Entry)(nil)
	lineNumber := 0
	for _, raw := range strings.Split(text, "\n") {
		lineNumber++
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "Windows Registry Editor") {
			continue
		}
		if strings.HasPrefix(line, "[") {

			flushPuttyEntry(&result, &current)
			key := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			name := puttySessionName(key)
			if name == "" {
				continue
			}
			current = &Entry{Alias: name, Port: 22, Line: lineNumber}
			continue
		}
		if current == nil {
			continue
		}
		name, value, ok := splitRegValue(line)
		if !ok {
			continue
		}
		switch strings.ToLower(name) {
		case "hostname":
			current.HostName = value
		case "username":
			current.User = value
		case "portnumber":
			if port, err := strconv.Atoi(value); err == nil && port > 0 && port <= 65535 {
				current.Port = port
			}
		}
	}
	flushPuttyEntry(&result, &current)
	return result, nil
}

func ParsePuttyFile(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read putty export: %w", err)
	}
	return ParsePuttyReg(data)
}

func DefaultPuttyRegPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "putty.reg")
}

func puttySessionName(key string) string {
	lower := strings.ToLower(key)
	index := strings.Index(lower, `sessions\`)
	if index < 0 {
		return ""
	}
	name := key[index+len(`sessions\`):]
	if name == "" || strings.Contains(name, `\`) {
		return ""
	}
	return strings.TrimSpace(name)
}

func splitRegValue(line string) (string, string, bool) {
	equals := strings.Index(line, "=")
	if equals <= 0 {
		return "", "", false
	}
	name := strings.Trim(strings.TrimSpace(line[:equals]), `"`)
	value := strings.TrimSpace(line[equals+1:])
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "dword:"):
		hexValue := strings.TrimSpace(value[len("dword:"):])
		parsed, err := strconv.ParseUint(hexValue, 16, 32)
		if err != nil {
			return name, "", false
		}
		return name, strconv.FormatUint(parsed, 10), true
	case strings.HasPrefix(value, `"`):
		return name, strings.Trim(value, `"`), true
	default:
		return "", "", false
	}
}

func decodeRegText(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
		}
		return string(utf16.Decode(units))
	}
	return strings.TrimPrefix(string(data), "\uFEFF")
}

func flushPuttyEntry(result *Result, current **Entry) {
	entry := *current
	if entry == nil {
		return
	}
	*current = nil
	if entry.HostName == "" {
		entry.HostName = entry.Alias
	}
	if entry.HostName == "" {
		result.Skipped++
		return
	}
	result.Entries = append(result.Entries, *entry)
}
