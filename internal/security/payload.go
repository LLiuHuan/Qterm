package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

const (
	payloadMagic   = "QTD1"
	payloadVersion = byte(1)
	maxPayloadSize = 16 << 20
)

type PayloadCipher struct {
	aead cipher.AEAD
}

func NewPayloadCipher(key []byte) (*PayloadCipher, error) {
	if len(key) != DEKSize {
		return nil, ErrInvalidDEK
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create local payload cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create local payload AEAD: %w", err)
	}
	return &PayloadCipher{aead: aead}, nil
}

func (c *PayloadCipher) Seal(domain, id string, plaintext []byte) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, ErrInvalidDEK
	}
	if domain == "" || id == "" || len(plaintext) > maxPayloadSize {
		return nil, ErrInvalidEnvelope
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(nonce)
		return nil, err
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, payloadAAD(domain, id))
	result := make([]byte, 0, len(payloadMagic)+1+len(nonce)+len(sealed))
	result = append(result, payloadMagic...)
	result = append(result, payloadVersion)
	result = append(result, nonce...)
	result = append(result, sealed...)
	clear(nonce)
	return result, nil
}

func (c *PayloadCipher) Open(domain, id string, envelope []byte) ([]byte, error) {
	if c == nil || c.aead == nil || domain == "" || id == "" {
		return nil, ErrInvalidEnvelope
	}
	minimum := len(payloadMagic) + 1 + c.aead.NonceSize() + c.aead.Overhead()
	if len(envelope) < minimum || len(envelope) > minimum+maxPayloadSize {
		return nil, ErrInvalidEnvelope
	}
	if string(envelope[:len(payloadMagic)]) != payloadMagic || envelope[len(payloadMagic)] != payloadVersion {
		return nil, ErrUnsupportedEnvelope
	}
	nonceStart := len(payloadMagic) + 1
	nonceEnd := nonceStart + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, envelope[nonceStart:nonceEnd], envelope[nonceEnd:], payloadAAD(domain, id))
	if err != nil {
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

func payloadAAD(domain, id string) []byte {
	hash := sha256.New()
	_, _ = io.WriteString(hash, "qterm/local-payload/v1/")
	_, _ = io.WriteString(hash, domain)
	_, _ = io.WriteString(hash, "/")
	_, _ = io.WriteString(hash, id)
	return hash.Sum(nil)
}
