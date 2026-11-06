package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	MasterKeySize = 32
	MaxSecretSize = 16 << 20

	envelopeMagic       = "QTVL"
	envelopeVersion     = byte(1)
	algorithmAES256GCM  = byte(1)
	envelopeHeaderSize  = len(envelopeMagic) + 2
	maxRecordIDSize     = 1024
	additionalDataLabel = "qterm-vault-record"
)

var (
	ErrInvalidKey           = errors.New("invalid vault master key")
	ErrNotReady             = errors.New("vault cipher is not initialized")
	ErrInvalidRecord        = errors.New("invalid vault record")
	ErrSecretTooLarge       = errors.New("vault secret is too large")
	ErrUnsupportedVersion   = errors.New("unsupported vault envelope version")
	ErrUnsupportedAlgorithm = errors.New("unsupported vault encryption algorithm")
	ErrAuthentication       = errors.New("vault record authentication failed")
)

type Kind byte

const (
	KindPassword Kind = iota + 1
	KindPrivateKey
	KindPrivateKeyPassphrase
	KindSSHAgent
)

type Cipher struct {
	aead   cipher.AEAD
	random io.Reader
}

func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, MasterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		clear(key)
		return nil, fmt.Errorf("generate vault master key: %w", err)
	}
	return key, nil
}

func NewCipher(masterKey []byte) (*Cipher, error) {
	return newCipher(masterKey, rand.Reader)
}

func newCipher(masterKey []byte, random io.Reader) (*Cipher, error) {
	if len(masterKey) != MasterKeySize {
		return nil, ErrInvalidKey
	}
	if random == nil {
		return nil, errors.New("vault random source is nil")
	}

	key := bytes.Clone(masterKey)
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create vault block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create vault AEAD: %w", err)
	}
	return &Cipher{aead: aead, random: random}, nil
}

func (c *Cipher) Seal(recordID string, kind Kind, plaintext []byte) ([]byte, error) {
	if c == nil || c.aead == nil || c.random == nil {
		return nil, ErrNotReady
	}
	if err := validateMetadata(recordID, kind); err != nil {
		return nil, err
	}
	if len(plaintext) > MaxSecretSize {
		return nil, ErrSecretTooLarge
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(c.random, nonce); err != nil {
		clear(nonce)
		return nil, fmt.Errorf("generate vault record nonce: %w", err)
	}

	additionalData := encodeAdditionalData(recordID, kind)
	sealed := c.aead.Seal(nil, nonce, plaintext, additionalData)
	envelope := make([]byte, 0, envelopeHeaderSize+len(nonce)+len(sealed))
	envelope = append(envelope, envelopeMagic...)
	envelope = append(envelope, envelopeVersion, algorithmAES256GCM)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, sealed...)
	clear(nonce)
	return envelope, nil
}

func (c *Cipher) Open(recordID string, kind Kind, envelope []byte) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, ErrNotReady
	}
	if err := validateMetadata(recordID, kind); err != nil {
		return nil, err
	}
	if err := c.validateEnvelope(envelope); err != nil {
		return nil, err
	}

	nonceStart := envelopeHeaderSize
	nonceEnd := nonceStart + c.aead.NonceSize()
	nonce := envelope[nonceStart:nonceEnd]
	ciphertext := envelope[nonceEnd:]
	additionalData := encodeAdditionalData(recordID, kind)
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

func (c *Cipher) validateEnvelope(envelope []byte) error {
	minimumSize := envelopeHeaderSize + c.aead.NonceSize() + c.aead.Overhead()
	if len(envelope) < minimumSize {
		return ErrInvalidRecord
	}
	if len(envelope) > envelopeHeaderSize+c.aead.NonceSize()+c.aead.Overhead()+MaxSecretSize {
		return ErrSecretTooLarge
	}
	if string(envelope[:len(envelopeMagic)]) != envelopeMagic {
		return ErrInvalidRecord
	}
	if envelope[len(envelopeMagic)] != envelopeVersion {
		return ErrUnsupportedVersion
	}
	if envelope[len(envelopeMagic)+1] != algorithmAES256GCM {
		return ErrUnsupportedAlgorithm
	}
	return nil
}

func validateMetadata(recordID string, kind Kind) error {
	if recordID == "" || len(recordID) > maxRecordIDSize || !kind.valid() {
		return ErrInvalidRecord
	}
	return nil
}

func (k Kind) valid() bool {
	return k >= KindPassword && k <= KindSSHAgent
}

func encodeAdditionalData(recordID string, kind Kind) []byte {
	additionalData := make([]byte, 0, len(additionalDataLabel)+len(envelopeMagic)+len(recordID)+12)
	additionalData = appendLengthPrefixed(additionalData, additionalDataLabel)
	additionalData = append(additionalData, envelopeMagic...)
	additionalData = append(additionalData, envelopeVersion, algorithmAES256GCM, byte(kind))
	additionalData = appendLengthPrefixed(additionalData, recordID)
	return additionalData
}

func appendLengthPrefixed(dst []byte, value string) []byte {
	dst = binary.BigEndian.AppendUint32(dst, uint32(len(value)))
	return append(dst, value...)
}
