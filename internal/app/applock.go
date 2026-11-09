package app

import (
	"context"
	"errors"

	"changeme/internal/applock"
	"changeme/internal/biometric"
	"changeme/internal/errno"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const appLockTouchIDReason = "Unlock Qterm"

func (s *Service) GetAppLockStatus() (applock.Status, error) {
	return s.appLock.Status(), nil
}

func (s *Service) EnableAppLock(password string, autoLockMinutes int, question, answer string) error {
	return s.appLock.Enable(password, autoLockMinutes, question, answer)
}

func (s *Service) ResetAppLockPassword(answer, next string) error {
	return s.appLock.ResetPasswordByRecovery(answer, next)
}

func (s *Service) NotifyAppLockState(locked bool) error {
	if locked && s.agents != nil {

		s.agents.CancelAll()
	}
	application.Get().Event.Emit(appLockEventName, locked)
	return nil
}

func (s *Service) ChangeAppLockPassword(current, next string) error {
	return s.appLock.ChangePassword(current, next)
}

func (s *Service) DisableAppLock(password string) error {
	return s.appLock.Disable(password)
}

func (s *Service) SetAppLockAutoLockMinutes(minutes int) error {
	return s.appLock.SetAutoLockMinutes(minutes)
}

func (s *Service) VerifyAppLockPassword(password string) (bool, error) {
	return s.appLock.Verify(password), nil
}

func (s *Service) SetAppLockTouchIDEnabled(enabled bool) error {
	return s.appLock.SetTouchIDEnabled(enabled)
}

func (s *Service) VerifyAppLockTouchID(ctx context.Context) (bool, error) {
	if !s.appLock.TouchIDEnabled() {
		return false, errno.New(errno.CodePermissionDenied, "applock.touchIdDisabled", "")
	}
	if !biometric.Available() {
		return false, errno.New(errno.CodeInvalidArg, "applock.touchIdUnavailable", "")
	}
	if err := biometric.Unlock(ctx, appLockTouchIDReason); err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return false, errno.New(errno.CodeCancelled, "applock.touchIdCancelled", "")
		case errors.Is(err, biometric.ErrUnavailable):
			return false, errno.New(errno.CodeInvalidArg, "applock.touchIdUnavailable", "")
		case errors.Is(err, biometric.ErrTimeout):
			return false, errno.New(errno.CodeTimeout, "applock.touchIdTimeout", "")
		default:

			return false, errno.New(errno.CodePermissionDenied, "applock.touchIdFailed", "")
		}
	}
	return true, nil
}
