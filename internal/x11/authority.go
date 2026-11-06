package x11

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
)

type authorityEntry struct {
	number string
	name   string
	data   []byte
}

var errAuthorityUnreadable = errors.New("authority file is unreadable")

func parseAuthorityFile(path string) ([]authorityEntry, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errAuthorityUnreadable
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make([]authorityEntry, 0, 4)
	for {
		var family uint16
		if err := binary.Read(file, binary.BigEndian, &family); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if _, err := readAuthString(file); err != nil {
			return nil, err
		}
		number, err := readAuthString(file)
		if err != nil {
			return nil, err
		}
		name, err := readAuthString(file)
		if err != nil {
			return nil, err
		}
		data, err := readAuthBytes(file)
		if err != nil {
			return nil, err
		}
		entries = append(entries, authorityEntry{number: number, name: name, data: data})
	}
	return entries, nil
}

func readAuthString(reader io.Reader) (string, error) {
	value, err := readAuthBytes(reader)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func readAuthBytes(reader io.Reader) ([]byte, error) {
	var size uint16
	if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}
	buffer := make([]byte, size)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func pickCookie(entries []authorityEntry, display string, screen int) (string, bool) {
	suffix := ":" + itoa(screen)
	var fallback string
	for _, entry := range entries {
		if !strings.EqualFold(entry.name, "MIT-MAGIC-COOKIE-1") || len(entry.data) == 0 {
			continue
		}
		if entry.number == display {
			return hexEncode(entry.data), true
		}
		if strings.HasSuffix(entry.number, suffix) {
			if fallback == "" {
				fallback = hexEncode(entry.data)
			}
		}
		if fallback == "" {
			fallback = hexEncode(entry.data)
		}
	}
	if fallback != "" {
		return fallback, true
	}
	return "", false
}

func hexEncode(value []byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, 0, len(value)*2)
	for _, item := range value {
		encoded = append(encoded, digits[item>>4], digits[item&0x0f])
	}
	return string(encoded)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buffer[index] = '-'
	}
	return string(buffer[index:])
}
