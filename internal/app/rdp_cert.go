package app

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/remotedesktop"
)

func (s *Service) rdpCertificateStore() (*remotedesktop.CertificateStore, error) {
	if s == nil || s.rdpCertificates == nil {
		return nil, rdpError(errno.CodeStoreFailed, "rdp.trustStoreFailed", nil)
	}
	return s.rdpCertificates, nil
}

func classifyRdpCertificateStoreError(err error) error {
	switch {
	case errors.Is(err, remotedesktop.ErrCertificateFingerprintInvalid):
		return rdpError(errno.CodeInvalidArg, "rdp.certificateUntrusted", err)
	case errors.Is(err, remotedesktop.ErrCertificateStoreCorrupt),
		errors.Is(err, remotedesktop.ErrCertificateStoreUnavailable):
		return rdpError(errno.CodeStoreFailed, "rdp.trustStoreFailed", err)
	}
	return rdpError(errno.CodeStoreFailed, "rdp.trustStoreFailed", err)
}

func (s *Service) rdpHostCertificateTarget(ctx context.Context, hostID string) (model.RdpHostProfile, error) {
	if s == nil || s.hosts == nil {
		return model.RdpHostProfile{}, rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	return s.rdpHostProfile(ctx, hostID)
}

func (s *Service) trustedRdpFingerprint(address string, port int) (string, error) {
	store, err := s.rdpCertificateStore()
	if err != nil {
		return "", err
	}
	record, found, err := store.LookupChecked(address, port)
	if err != nil {
		return "", classifyRdpCertificateStoreError(err)
	}
	if !found {
		return "", nil
	}
	return record.Fingerprint, nil
}

func (s *Service) liveRdpCertificateDetails(address string, port int, fingerprint string) (subject, issuer string, ok bool) {
	if s == nil || s.remoteDesktops == nil {
		return "", "", true
	}
	session, found := s.remoteDesktops.FindByHost(address, port)
	if !found {
		return "", "", true
	}
	status := session.CertificateStatus()
	live := remotedesktop.NormalizeCertificateFingerprint(status.Fingerprint)
	if live == "" {
		return "", "", true
	}
	if live != fingerprint {
		return "", "", false
	}
	return strings.TrimSpace(status.Subject), strings.TrimSpace(status.Issuer), true
}

func (s *Service) RdpTrustCertificate(hostID string, fingerprint string) error {

	normalized := remotedesktop.NormalizeCertificateFingerprint(fingerprint)
	if normalized == "" {
		return publicError(rdpError(errno.CodeInvalidArg, "rdp.certificateUntrusted", nil))
	}
	ctx := context.Background()
	profile, err := s.rdpHostCertificateTarget(ctx, hostID)
	if err != nil {
		return publicError(err)
	}
	store, err := s.rdpCertificateStore()
	if err != nil {
		return publicError(err)
	}
	subject, issuer, ok := s.liveRdpCertificateDetails(profile.Address, profile.Port, normalized)
	if !ok {

		return publicError(rdpError(errno.CodeInvalidArg, "rdp.certificateChanged", nil))
	}
	if err := store.Trust(profile.Address, profile.Port, normalized, subject, issuer); err != nil {
		return publicError(classifyRdpCertificateStoreError(err))
	}
	return nil
}

func (s *Service) RdpForgetCertificate(hostID string) error {
	ctx := context.Background()
	profile, err := s.rdpHostCertificateTarget(ctx, hostID)
	if err != nil {
		return publicError(err)
	}
	store, err := s.rdpCertificateStore()
	if err != nil {
		return publicError(err)
	}
	if err := store.Forget(profile.Address, profile.Port); err != nil {
		return publicError(classifyRdpCertificateStoreError(err))
	}
	return nil
}

func (s *Service) RdpTrustedCertificate(hostID string) (model.RdpTrustedCertificate, error) {
	ctx := context.Background()
	profile, err := s.rdpHostCertificateTarget(ctx, hostID)
	if err != nil {
		return model.RdpTrustedCertificate{}, publicError(err)
	}
	store, err := s.rdpCertificateStore()
	if err != nil {
		return model.RdpTrustedCertificate{}, publicError(err)
	}
	record, found, err := store.LookupChecked(profile.Address, profile.Port)
	if err != nil {
		return model.RdpTrustedCertificate{}, publicError(classifyRdpCertificateStoreError(err))
	}
	result := model.RdpTrustedCertificate{
		HostID:  profile.HostID,
		Address: profile.Address,
		Port:    profile.Port,
	}
	if !found {
		return result, nil
	}
	result.Fingerprint = record.Fingerprint
	result.Subject = record.Subject
	result.Issuer = record.Issuer
	result.TrustedAt = record.TrustedAt
	return result, nil
}
