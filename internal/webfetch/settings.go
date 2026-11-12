package webfetch

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const (
	ModeSystem = "system"

	ModeDirect = "direct"

	ModeCustom = "custom"
)

const PreferenceKey = "qterm.webfetch.v1"

type Settings struct {
	Mode string `json:"mode"`

	ProxyURL string `json:"proxyUrl,omitempty"`

	AllowPrivate bool `json:"allowPrivate"`
}

func DefaultSettings() Settings {
	return Settings{Mode: ModeSystem}
}

func Normalize(settings Settings) Settings {
	normalized := Settings{
		Mode:         strings.TrimSpace(settings.Mode),
		ProxyURL:     strings.TrimSpace(settings.ProxyURL),
		AllowPrivate: settings.AllowPrivate,
	}
	if normalized.Mode != ModeDirect && normalized.Mode != ModeCustom {
		normalized.Mode = ModeSystem
	}
	if normalized.Mode == ModeCustom && !validProxyURL(normalized.ProxyURL) {
		normalized.Mode = ModeSystem
		normalized.ProxyURL = ""
	}
	if normalized.Mode != ModeCustom {
		normalized.ProxyURL = ""
	}
	return normalized
}

func validProxyURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
		return true
	default:
		return false
	}
}

func EncodeSettings(settings Settings) (string, error) {
	encoded, err := json.Marshal(Normalize(settings))
	if err != nil {
		return "", err
	}
	return string(encoded), nil
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

var (
	ErrBlockedAddress = errors.New("webfetch: target address is blocked")

	ErrInvalidURL = errors.New("webfetch: invalid url")

	ErrUnsupportedContent = errors.New("webfetch: unsupported content type")

	ErrTooLarge = errors.New("webfetch: response is too large")

	ErrTooManyRedirects = errors.New("webfetch: too many redirects")

	ErrUpstream = errors.New("webfetch: request failed")
)

type Result struct {
	URL string

	Status int

	ContentType string

	Title string

	Text string

	Truncated bool

	Bytes int64
}

func Fetch(ctx context.Context, settings Settings, rawURL string) (Result, error) {
	normalized := Normalize(settings)
	target, err := normalizeTarget(rawURL)
	if err != nil {
		return Result{}, err
	}
	client, err := newClient(normalized)
	if err != nil {
		return Result{}, err
	}
	return fetchWithClient(ctx, client, target)
}

func normalizeTarget(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len(trimmed) > 2048 {
		return nil, ErrInvalidURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return nil, ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL
	}

	parsed.Fragment = ""
	return parsed, nil
}
