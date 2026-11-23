package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/trzszfiles"
	"changeme/internal/zmodem"
)

func (s *Service) SelectTrzszUploads(ctx context.Context) ([]zmodem.LocalFile, error) {
	if s.fileDialogs == nil {
		return nil, errno.New(errno.CodeNotFound, "error.notFound", "")
	}
	paths, err := s.fileDialogs.ChooseUploadFiles()
	if err != nil {
		return nil, publicError(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, publicError(err)
	}
	files := make([]zmodem.LocalFile, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		file, openErr := s.zmodem.OpenTrzszUpload(ctx, path)
		if openErr != nil {

			s.releaseZmodemFiles(files)
			return nil, publicError(openErr)
		}
		files = append(files, file)
	}
	return files, nil
}

func (s *Service) SelectTrzszDownloadDirectory(ctx context.Context) (trzszfiles.Directory, error) {
	if s.fileDialogs == nil {
		return trzszfiles.Directory{}, errno.New(errno.CodeNotFound, "error.notFound", "")
	}
	path, err := s.fileDialogs.ChooseLocalDirectory()
	if err != nil {
		return trzszfiles.Directory{}, publicError(err)
	}
	if err := ctx.Err(); err != nil {
		return trzszfiles.Directory{}, publicError(err)
	}
	if path == "" {
		return trzszfiles.Directory{}, nil
	}
	directory, err := s.trzszFiles.OpenDirectory(path)
	return directory, publicError(err)
}

func (s *Service) ReleaseTrzszDownloadDirectory(token string) error {
	return publicError(s.trzszFiles.ReleaseDirectory(token))
}

func (s *Service) OpenTrzszDownloadFile(ctx context.Context, directoryToken, relativePath string, overwrite bool) (trzszfiles.File, error) {
	file, err := s.trzszFiles.OpenDownload(ctx, directoryToken, relativePath, overwrite)
	return file, publicError(err)
}

func (s *Service) CreateTrzszDirectory(ctx context.Context, directoryToken, relativePath string) error {
	return publicError(s.trzszFiles.CreateDirectory(ctx, directoryToken, relativePath))
}

func (s *Service) WriteTrzszFile(ctx context.Context, token string, offset int64, data []byte) error {
	return publicError(s.trzszFiles.Write(ctx, token, offset, data))
}

func (s *Service) CommitTrzszFile(ctx context.Context, token string) (trzszfiles.File, error) {
	file, err := s.trzszFiles.Commit(ctx, token)
	return file, publicError(err)
}

func (s *Service) AbortTrzszFile(token string) error {
	return publicError(s.trzszFiles.Abort(token))
}

func (s *Service) releaseZmodemFiles(files []zmodem.LocalFile) {
	for _, file := range files {
		_ = s.zmodem.Release(file.Token)
	}
}
