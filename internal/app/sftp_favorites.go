package app

import (
	"context"
	"encoding/json"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	sftpFavoritesPrefix = "qterm.sftp.favorites.v1."

	sftpFavoritesLimit = 20
)

func sftpFavoritesKey(hostID string) (string, error) {
	trimmed := strings.TrimSpace(hostID)
	if trimmed == "" {
		return "", errno.New(errno.CodeInvalidArg, "sftp.favorites.hostRequired", "")
	}
	return sftpFavoritesPrefix + trimmed, nil
}

func normalizeSFTPFavorites(items []model.SFTPFavoritePath) []model.SFTPFavoritePath {
	result := make([]model.SFTPFavoritePath, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		cleaned, ok := model.CleanSFTPFavoritePath(item.Path)
		if !ok || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		result = append(result, model.SFTPFavoritePath{
			Path: cleaned,
			Name: model.NormalizeSFTPFavoriteName(item.Name),
		})
		if len(result) >= sftpFavoritesLimit {
			break
		}
	}
	return result
}

func decodeSFTPFavorites(raw string) []model.SFTPFavoritePath {
	var structured []model.SFTPFavoritePath
	if err := json.Unmarshal([]byte(raw), &structured); err == nil && len(structured) > 0 {
		return structured
	}
	var legacy []string
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil
	}
	converted := make([]model.SFTPFavoritePath, 0, len(legacy))
	for _, item := range legacy {
		converted = append(converted, model.SFTPFavoritePath{Path: item})
	}
	return converted
}

func (s *Service) ListSFTPFavorites(hostID string) ([]model.SFTPFavoritePath, error) {
	key, err := sftpFavoritesKey(hostID)
	if err != nil {
		return nil, publicError(err)
	}
	if s == nil || s.preferences == nil {
		return []model.SFTPFavoritePath{}, nil
	}
	raw, found, err := s.preferences.Get(context.Background(), key)
	if err != nil {
		return nil, publicError(errno.Wrap(errno.CodeStoreFailed, "sftp.favorites.readFailed", err))
	}
	if !found || strings.TrimSpace(raw) == "" {
		return []model.SFTPFavoritePath{}, nil
	}

	return normalizeSFTPFavorites(decodeSFTPFavorites(raw)), nil
}

func (s *Service) SetSFTPFavorites(hostID string, favorites []model.SFTPFavoritePath) error {
	key, err := sftpFavoritesKey(hostID)
	if err != nil {
		return publicError(err)
	}
	if s == nil || s.preferences == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	normalized := normalizeSFTPFavorites(favorites)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return publicError(errno.Wrap(errno.CodeUnknown, "sftp.favorites.writeFailed", err))
	}
	if err := s.preferences.Set(context.Background(), key, string(encoded)); err != nil {
		return publicError(errno.Wrap(errno.CodeStoreFailed, "sftp.favorites.writeFailed", err))
	}
	return nil
}
