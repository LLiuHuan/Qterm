package hostimport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var ErrImportLocked = errors.New("import source locked")

func decodeLegacyText(raw []byte) string {
	if len(raw) >= 2 {
		switch {
		case raw[0] == 0xFF && raw[1] == 0xFE:
			return decodeUTF16(raw[2:], binary.LittleEndian)
		case raw[0] == 0xFE && raw[1] == 0xFF:
			return decodeUTF16(raw[2:], binary.BigEndian)
		}
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(raw) {
		return string(raw)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {

		return string(raw)
	}
	return string(decoded)
}

func decodeUTF16(raw []byte, order binary.ByteOrder) string {
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, order.Uint16(raw[index:index+2]))
	}
	return string(utf16.Decode(units))
}

type iniSection struct {
	name   string
	values map[string]string
	keys   []string
}

func (s *iniSection) value(key string) string {
	if s == nil {
		return ""
	}
	return s.values[key]
}

type iniDocument struct {
	sections []*iniSection
	byName   map[string]*iniSection
}

func (d *iniDocument) section(name string) *iniSection {
	if d == nil {
		return nil
	}
	return d.byName[name]
}

func parseINISections(content string) *iniDocument {
	document := &iniDocument{byName: make(map[string]*iniSection, 8)}
	var current *iniSection
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			if existing, ok := document.byName[name]; ok {
				current = existing
				continue
			}
			current = &iniSection{name: name, values: make(map[string]string, 8)}
			document.sections = append(document.sections, current)
			document.byName[name] = current
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || current == nil {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := current.values[key]; !exists {
			current.keys = append(current.keys, key)
		}
		current.values[key] = strings.TrimSpace(value)
	}
	return document
}

func joinGroupSegments(segments []string) string {
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		if trimmed := strings.TrimSpace(segment); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "/")
}
