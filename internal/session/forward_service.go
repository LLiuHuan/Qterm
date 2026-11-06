package session

import "changeme/internal/model"

func (s *Service) SetForwardEmitter(emit func(model.ForwardStatus)) {
	if s == nil || s.forwards == nil {
		return
	}
	s.forwards.SetEmitter(emit)
}

func (s *Service) OpenForward(config model.ForwardConfig) error {
	if s == nil || s.forwards == nil {
		return ErrServiceNotReady
	}
	return s.forwards.Open(config)
}

func (s *Service) CloseForward(id string) error {
	if s == nil || s.forwards == nil {
		return ErrServiceNotReady
	}
	return s.forwards.Close(id)
}

func (s *Service) ForwardStatus(id string) model.ForwardStatus {
	if s == nil || s.forwards == nil {
		return model.ForwardStatus{ID: id, State: model.ForwardStopped}
	}
	return s.forwards.Status(id)
}

func (s *Service) ActiveForwardCount() int {
	if s == nil || s.forwards == nil {
		return 0
	}
	return s.forwards.ActiveCount()
}
