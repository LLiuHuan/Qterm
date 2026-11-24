package remotedesktop

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func decodeHelperConsoleText(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	if looksLikeUTF16LE(raw) {
		if text, ok := decodeUTF16LE(raw); ok {
			return text
		}
	}
	if text, ok := decodeGBK(raw); ok {
		return text
	}
	return string(raw)
}

func looksLikeUTF16LE(raw []byte) bool {
	if len(raw) < 2 || len(raw)%2 != 0 {
		return false
	}
	units := len(raw) / 2
	hanHighBytes, zeroHighBytes := 0, 0
	for i := 1; i < len(raw); i += 2 {
		switch {
		case raw[i] == 0:
			zeroHighBytes++
		case raw[i] >= 0x4E && raw[i] <= 0x9F:
			hanHighBytes++
		}
	}

	if zeroHighBytes > 0 && hanHighBytes > 0 {
		return true
	}
	return hanHighBytes*2 >= units
}

func decodeUTF16LE(raw []byte) (string, bool) {
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		units = append(units, uint16(raw[i])|uint16(raw[i+1])<<8)
	}
	text := string(utf16.Decode(units))
	if !isReadableText(text) {
		return "", false
	}
	return text, true
}

func decodeGBK(raw []byte) (string, bool) {
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return "", false
	}
	text := string(decoded)
	if !isReadableText(text) {
		return "", false
	}
	return text, true
}

func isReadableText(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r == utf8.RuneError || (unicode.IsControl(r) && r != '\t') {
			return false
		}
	}
	return strings.TrimSpace(text) != ""
}
