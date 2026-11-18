package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
)

var updatePublicKeyHexes = []string{
	"f02c63b4491d5bbe4715e48eb64c9ac8fcf7a40ab610884f0ea7cfbb8b7845cf",
}

const signatureAssetName = "SHA256SUMS.sig"

const maxSignatureBytes = 1 << 12

const signatureRequired = true

func VerifyChecksumsSignature(checksums, signature []byte) error {
	if !signatureRequired && len(signature) == 0 {
		return nil
	}
	if len(signature) == 0 {
		slog.Error("update checksums signature missing", "event", "update.signature.missing")
		return signatureMissing()
	}
	decoded, err := decodeSignature(signature)
	if err != nil {

		slog.Error("update checksums signature malformed",
			"event", "update.signature.malformed",
			"bytes", len(signature),
		)
		return signatureInvalid("format")
	}
	if len(checksums) == 0 {
		slog.Error("update checksums empty", "event", "update.signature.checksums_empty")
		return signatureInvalid("checksum")
	}
	for _, keyHex := range updatePublicKeyHexes {
		key, keyErr := hex.DecodeString(strings.TrimSpace(keyHex))
		if keyErr != nil || len(key) != ed25519.PublicKeySize {
			continue
		}
		if ed25519.Verify(ed25519.PublicKey(key), checksums, decoded) {
			return nil
		}
	}
	slog.Error("update checksums signature mismatch",
		"event", "update.signature.mismatch",
		"bytes", len(checksums),
	)
	return signatureInvalid("mismatch")
}

func decodeSignature(raw []byte) ([]byte, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, errors.New("empty signature")
	}
	if len(text) == ed25519.SignatureSize*2 && isHex(text) {
		return hex.DecodeString(text)
	}
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	return nil, errors.New("unrecognized signature encoding")
}
