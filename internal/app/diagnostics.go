package app

import "errors"

func (s *Service) DebugLoggingEnabled() bool {
	return s != nil && s.diagnostics != nil && s.diagnostics.Enabled()
}

func (s *Service) SetDebugLoggingEnabled(enabled bool) error {
	if s == nil || s.diagnostics == nil {
		return errors.New("调试日志未配置")
	}
	return publicError(s.diagnostics.SetEnabled(enabled))
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
