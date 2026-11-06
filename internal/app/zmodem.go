package app

import (
	"context"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/zmodem"
)

func (s *Service) SelectZmodemUpload(ctx context.Context) (zmodem.LocalFile, error) {
	if s.fileDialogs == nil {
		return zmodem.LocalFile{}, errno.New(errno.CodeNotFound, "error.notFound", "")
	}
	path, err := s.fileDialogs.ChooseUploadFile()
	if err != nil {
		return zmodem.LocalFile{}, publicError(err)
	}
	if err := ctx.Err(); err != nil {
		return zmodem.LocalFile{}, publicError(err)
	}
	if path == "" {
		return zmodem.LocalFile{}, nil
	}
	file, err := s.zmodem.OpenUpload(ctx, path)
	return file, publicError(err)
}

func (s *Service) SelectZmodemDownload(ctx context.Context, name string, size int64) (zmodem.LocalFile, error) {
	if s.fileDialogs == nil {
		return zmodem.LocalFile{}, errno.New(errno.CodeNotFound, "error.notFound", "")
	}

	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == ".." || name == "" {
		name = "download"
	}
	path, err := s.fileDialogs.ChooseDownloadFile(name)
	if err != nil {
		return zmodem.LocalFile{}, publicError(err)
	}
	if err := ctx.Err(); err != nil {
		return zmodem.LocalFile{}, publicError(err)
	}
	if path == "" {
		return zmodem.LocalFile{}, nil
	}
	file, err := s.zmodem.CreateDownload(ctx, path, size)
	return file, publicError(err)
}

func (s *Service) ReadZmodemFile(ctx context.Context, token string, offset int64, length int) ([]byte, error) {
	data, err := s.zmodem.Read(ctx, token, offset, length)
	return data, publicError(err)
}

func (s *Service) WriteZmodemFile(ctx context.Context, token string, offset int64, data []byte) error {
	return publicError(s.zmodem.Write(ctx, token, offset, data))
}

func (s *Service) FinishZmodemFile(ctx context.Context, token string) error {
	return publicError(s.zmodem.Finish(ctx, token))
}

func (s *Service) ReleaseZmodemFile(token string) error { return publicError(s.zmodem.Release(token)) }

func (s *Service) ResolveZmodemDownload(ctx context.Context, token, action string) (zmodem.LocalFile, error) {
	file, err := s.zmodem.ResolveDownload(ctx, token, action)
	return file, publicError(err)
}
