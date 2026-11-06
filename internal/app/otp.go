package app

import (
	"context"
	"os"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/otp"
	"changeme/internal/security"
)

var errOTPNotReady = errno.New(errno.CodeInvalidArg, "otp.service_unavailable", "")

func newOTPService(manager *security.Manager) *otp.Service {
	return otp.NewService(os.UserConfigDir, manager.DataKey)
}

func (s *Service) SaveOTPEntry(input model.OTPSaveInput) (model.OTPEntry, error) {
	if s == nil || s.otp == nil {
		return model.OTPEntry{}, publicError(errOTPNotReady)
	}
	entry, err := s.otp.Save(context.Background(), input)
	if err != nil {
		return model.OTPEntry{}, publicError(err)
	}
	return entry, nil
}

func (s *Service) DeleteOTPEntry(id string) error {
	if s == nil || s.otp == nil {
		return publicError(errOTPNotReady)
	}
	return publicError(s.otp.Delete(context.Background(), id))
}

func (s *Service) GenerateOTPCode(entry model.OTPEntry) (model.OTPCode, error) {
	if s == nil || s.otp == nil {
		return model.OTPCode{}, publicError(errOTPNotReady)
	}
	code, err := s.otp.Generate(context.Background(), entry)
	if err != nil {
		return model.OTPCode{}, publicError(err)
	}
	return code, nil
}

func (s *Service) OTPProvisioningURI(entry model.OTPEntry) (string, error) {
	if s == nil || s.otp == nil {
		return "", publicError(errOTPNotReady)
	}
	uri, err := s.otp.ProvisioningURI(context.Background(), entry)
	if err != nil {
		return "", publicError(err)
	}
	return uri, nil
}
