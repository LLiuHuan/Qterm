package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	xssh "golang.org/x/crypto/ssh"
)

const userCertificatePayloadPrefix = "qterm-user-certificate-v1:"

var (
	ErrInvalidUserCertificate = errors.New("invalid user certificate")
)

type userCertificatePayload struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"privateKey"`
	Passphrase  string `json:"passphrase,omitempty"`
}

type UserCertificatePayload struct {
	Certificate string
	PrivateKey  string
	Passphrase  string
}

func EncodeUserCertificatePayload(certificate, privateKey, passphrase string) ([]byte, error) {
	certificate = strings.TrimSpace(certificate)
	privateKey = strings.TrimSpace(privateKey)
	if certificate == "" || privateKey == "" {
		return nil, ErrInvalidUserCertificate
	}
	payload, err := json.Marshal(userCertificatePayload{
		Certificate: certificate,
		PrivateKey:  privateKey,
		Passphrase:  passphrase,
	})
	if err != nil {
		return nil, ErrInvalidUserCertificate
	}
	return append([]byte(userCertificatePayloadPrefix), payload...), nil
}

func DecodeUserCertificatePayload(secret []byte) (UserCertificatePayload, error) {
	if !bytes.HasPrefix(secret, []byte(userCertificatePayloadPrefix)) {
		return UserCertificatePayload{}, ErrInvalidUserCertificate
	}
	var payload userCertificatePayload
	if err := json.Unmarshal(secret[len(userCertificatePayloadPrefix):], &payload); err != nil {
		return UserCertificatePayload{}, ErrInvalidUserCertificate
	}
	if strings.TrimSpace(payload.Certificate) == "" || strings.TrimSpace(payload.PrivateKey) == "" {
		return UserCertificatePayload{}, ErrInvalidUserCertificate
	}
	return UserCertificatePayload{
		Certificate: payload.Certificate,
		PrivateKey:  payload.PrivateKey,
		Passphrase:  payload.Passphrase,
	}, nil
}

func ParseUserCertificate(text string) (*xssh.Certificate, error) {
	text = strings.TrimSpace(text)
	if text == "" || !utf8.ValidString(text) || len(text) > MaxCredentialSecretSize {
		return nil, ErrInvalidUserCertificate
	}
	publicKey, _, _, _, err := xssh.ParseAuthorizedKey([]byte(text))
	if err != nil {
		return nil, ErrInvalidUserCertificate
	}
	certificate, ok := publicKey.(*xssh.Certificate)
	if !ok {
		return nil, ErrInvalidUserCertificate
	}
	return certificate, nil
}

func NormalizeUserCertificateText(text string) (string, error) {
	certificate, err := ParseUserCertificate(text)
	if err != nil {
		return "", err
	}
	normalized := strings.TrimSpace(string(xssh.MarshalAuthorizedKey(certificate)))
	if normalized == "" {
		return "", ErrInvalidUserCertificate
	}
	return normalized, nil
}
