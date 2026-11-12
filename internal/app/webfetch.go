package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/webfetch"
)

func (s *Service) WebFetchSettings() (webfetch.Settings, error) {
	if s == nil || s.preferences == nil {
		return webfetch.DefaultSettings(), nil
	}
	raw, found, err := s.preferences.Get(context.Background(), webfetch.PreferenceKey)
	if err != nil {
		return webfetch.DefaultSettings(), publicError(err)
	}
	if !found {
		return webfetch.DefaultSettings(), nil
	}
	return webfetch.DecodeSettings(raw), nil
}

func (s *Service) SetWebFetchSettings(settings webfetch.Settings) error {
	if s == nil || s.preferences == nil {
		return errno.New(errno.CodeStoreFailed, "error.serviceNotReady", "")
	}
	mode := settings.Mode
	switch mode {
	case webfetch.ModeSystem, webfetch.ModeDirect, webfetch.ModeCustom:
	case "":
		mode = webfetch.ModeSystem
	default:
		return errno.New(errno.CodeInvalidArg, "settings.ai.web.invalidMode", "")
	}
	normalized := webfetch.Normalize(webfetch.Settings{
		Mode:         mode,
		ProxyURL:     settings.ProxyURL,
		AllowPrivate: settings.AllowPrivate,
	})
	if mode == webfetch.ModeCustom && normalized.Mode != webfetch.ModeCustom {
		return errno.New(errno.CodeInvalidArg, "settings.ai.web.invalidProxy", "")
	}
	encoded, err := webfetch.EncodeSettings(normalized)
	if err != nil {
		return errno.Wrap(errno.CodeUnknown, "settings.ai.web.saveFailed", err)
	}
	if err := s.preferences.Set(context.Background(), webfetch.PreferenceKey, encoded); err != nil {
		return publicError(err)
	}
	return nil
}
