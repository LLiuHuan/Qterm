package credential

import (
	"bytes"
	"strings"

	"changeme/internal/model"
	xssh "golang.org/x/crypto/ssh"
)

func prepareUserCertificateSecret(certificateText, privateKeyText, passphrase string) ([]byte, string, error) {
	certificate, err := model.ParseUserCertificate(certificateText)
	if err != nil {
		return nil, "", err
	}
	key, publicKey, err := normalizePrivateKey(privateKeyText, passphrase)
	if err != nil {
		return nil, "", err
	}
	defer clear(key)
	certified, _, _, _, parseErr := xssh.ParseAuthorizedKey([]byte(publicKey))
	if parseErr != nil {
		return nil, "", ErrInvalidCredentialSecret
	}
	if !bytes.Equal(certified.Marshal(), certificate.Key.Marshal()) {
		return nil, "", ErrInvalidCredentialSecret
	}
	normalizedCertificate, err := model.NormalizeUserCertificateText(certificateText)
	if err != nil {
		return nil, "", err
	}
	payload, err := model.EncodeUserCertificatePayload(normalizedCertificate, string(key), passphrase)
	if err != nil {
		return nil, "", err
	}

	return payload, strings.TrimSpace(string(xssh.MarshalAuthorizedKey(certificate.Key))), nil
}
