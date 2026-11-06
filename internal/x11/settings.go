package x11

import (
	"encoding/json"
	"strings"
)

const PreferenceKey = "x11"

func DefaultSettings() Settings {
	return Settings{}
}

func Normalize(settings Settings) Settings {
	return Settings{
		Display:       strings.TrimSpace(settings.Display),
		XauthPath:     strings.TrimSpace(settings.XauthPath),
		AuthorityFile: strings.TrimSpace(settings.AuthorityFile),
	}
}

func DecodeSettings(raw string) Settings {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultSettings()
	}
	var decoded Settings
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return DefaultSettings()
	}
	return Normalize(decoded)
}

func EncodeSettings(settings Settings) (string, error) {
	encoded, err := json.Marshal(Normalize(settings))
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
