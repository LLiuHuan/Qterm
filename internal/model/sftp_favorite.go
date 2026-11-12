package model

import (
	"path"
	"strings"
	"unicode/utf8"
)

const MaxSFTPFavoriteNameSize = 64

type SFTPFavoritePath struct {
	Path string `json:"path"`

	Name string `json:"name,omitempty"`
}

func NormalizeSFTPFavoriteName(raw string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(raw) {
		if char == utf8.RuneError {
			continue
		}
		if char < 0x20 || char == 0x7f || char == '\u200b' || char == '\ufeff' {
			continue
		}
		builder.WriteRune(char)
	}
	runes := []rune(strings.TrimSpace(builder.String()))
	if len(runes) > MaxSFTPFavoriteNameSize {
		runes = runes[:MaxSFTPFavoriteNameSize]
	}
	return strings.TrimSpace(string(runes))
}

func DefaultSFTPFavoriteName(remotePath string) string {
	base := path.Base(remotePath)
	if base == "." || base == "/" || base == "" {
		return "根目录"
	}
	return base
}

func CleanSFTPFavoritePath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return "", false
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		return "", false
	}
	return cleaned, true
}
