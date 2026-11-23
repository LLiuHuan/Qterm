package app

import (
	"context"
	"errors"

	"changeme/internal/clitool"
	"changeme/internal/errno"
)

func (s *Service) CLIToolState(ctx context.Context) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	state, err := s.cliToolService().State()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) InstallCLIToolBinary(ctx context.Context) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	state, err := s.cliToolService().InstallBinary()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) UninstallCLIToolBinary(ctx context.Context) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	state, err := s.cliToolService().UninstallBinary()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) InstallCLIToolSkills(ctx context.Context) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	state, err := s.cliToolService().InstallSkills()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) UninstallCLIToolSkills(ctx context.Context) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	state, err := s.cliToolService().UninstallSkills()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) ApplyCLIToolSettings(ctx context.Context, settings clitool.Settings) (clitool.State, error) {
	if err := contextReady(ctx); err != nil {
		return clitool.State{}, err
	}
	service := s.cliToolService()
	if _, err := service.ApplySettings(settings); err != nil {
		return service.StateOrZero(), publicCLIToolError(err)
	}
	state, err := service.State()
	if err != nil {
		return state, publicCLIToolError(err)
	}
	return state, nil
}

func (s *Service) cliToolService() *clitool.Service {
	if s == nil {
		return clitool.NewService(nil)
	}
	return clitool.NewService(s.preferences)
}

func publicCLIToolError(err error) error {
	if err == nil {
		return nil
	}
	var keyed interface {
		MessageKey() string
		Detail() string
	}
	if errors.As(err, &keyed) {
		return errno.New(errno.CodeStoreFailed, keyed.MessageKey(), keyed.Detail())
	}
	return errno.New(errno.CodeStoreFailed, "settings.cli.error.generic", "")
}
