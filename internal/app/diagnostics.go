package app

import (
	"changeme/internal/diagnostics"
	"changeme/internal/errno"
	"errors"
)

type DebugLogStatus struct {
	Mode       string `json:"mode"`
	Path       string `json:"path"`
	SizeBytes  int64  `json:"sizeBytes"`
	LimitBytes int64  `json:"limitBytes"`
	Dropped    uint64 `json:"dropped"`
	Present    bool   `json:"present"`
}

func (s *Service) DebugLogMode() string {
	if s == nil || s.diagnostics == nil {
		return string(diagnostics.ModeOff)
	}
	return string(s.diagnostics.Mode())
}

func (s *Service) SetDebugLogMode(mode string) error {
	if s == nil || s.diagnostics == nil {
		return errors.New("调试日志未配置")
	}
	parsed, ok := diagnostics.ParseMode(mode)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "调试日志档位无效")
	}
	return publicError(s.diagnostics.SetMode(parsed))
}

func (s *Service) DebugLogStatus() (DebugLogStatus, error) {
	if s == nil || s.diagnostics == nil {
		return DebugLogStatus{}, errors.New("调试日志未配置")
	}
	status := s.diagnostics.Status()
	return DebugLogStatus{
		Mode:       string(status.Mode),
		Path:       status.Path,
		SizeBytes:  status.SizeBytes,
		LimitBytes: status.LimitBytes,
		Dropped:    status.Dropped,
		Present:    status.Present,
	}, nil
}

func (s *Service) ReadDebugLog() (string, error) {
	if s == nil || s.diagnostics == nil {
		return "", errors.New("调试日志未配置")
	}
	return publicValue(s.diagnostics.Read())
}

func (s *Service) ClearDebugLog() error {
	if s == nil || s.diagnostics == nil {
		return errors.New("调试日志未配置")
	}
	return publicError(s.diagnostics.Clear())
}

func (s *Service) ExportDebugLog() (bool, error) {
	if s == nil || s.diagnostics == nil || s.fileDialogs == nil {
		return false, errors.New("调试日志导出未配置")
	}
	target, err := s.fileDialogs.ChooseDebugLogDestination()
	if err != nil {
		return false, publicError(err)
	}
	if target == "" {
		return false, nil
	}
	if err := s.diagnostics.Export(target); err != nil {
		return false, publicError(err)
	}
	return true, nil
}
