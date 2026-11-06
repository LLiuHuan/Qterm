package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	customEnvelopeMagic   = "QTCK"
	recoveryEnvelopeMagic = "QTRP"
	envelopeVersion       = byte(1)
	envelopeKDFArgon2id   = byte(1)
	envelopeCipherAESGCM  = byte(1)

	envelopeArgonVersion = byte(argon2.Version)
	envelopeArgonTime    = uint32(3)
	envelopeArgonMemory  = uint32(64 * 1024)
	envelopeArgonThreads = uint8(4)
	envelopeSaltSize     = uint8(16)
	envelopeNonceSize    = uint8(12)
	envelopeTagSize      = 16

	envelopeHeaderSize    = len(customEnvelopeMagic) + 1 + 1 + 1 + 1 + 4 + 4 + 1 + 1 + 1
	envelopeSealedDEKSize = DEKSize + envelopeTagSize
	envelopeSize          = envelopeHeaderSize + int(envelopeSaltSize) + int(envelopeNonceSize) + envelopeSealedDEKSize
	maxEnvelopeSize       = envelopeSize
	customKeyAADLabel     = "qterm/custom-key-envelope/v1"
	recoveryAADLabel      = "qterm/recovery-package/v1"
)

type envelopeParameters struct {
	magic        string
	header       []byte
	salt         []byte
	nonce        []byte
	sealedDEK    []byte
	argonTime    uint32
	argonMemory  uint32
	argonThreads uint8
}

func wrapDEKWithKey(dek, key []byte, magic, aadLabel string, random io.Reader) ([]byte, error) {
	if !validDEK(dek) {
		return nil, ErrInvalidDEK
	}
	if err := validateCustomKey(key); err != nil {
		return nil, err
	}
	if random == nil {
		return nil, fmt.Errorf("generate encryption key envelope: random source is nil")
	}
	if len(magic) != len(customEnvelopeMagic) {
		return nil, ErrInvalidEnvelope
	}

	header := encodeEnvelopeHeader(magic)
	salt := make([]byte, envelopeSaltSize)
	if _, err := io.ReadFull(random, salt); err != nil {
		clear(salt)
		return nil, fmt.Errorf("generate encryption key salt: %w", err)
	}
	nonce := make([]byte, envelopeNonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		clear(salt)
		clear(nonce)
		return nil, fmt.Errorf("generate encryption key nonce: %w", err)
	}

	derivedKey := argon2.IDKey(
		key,
		salt,
		envelopeArgonTime,
		envelopeArgonMemory,
		envelopeArgonThreads,
		DEKSize,
	)
	defer clear(derivedKey)
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		clear(salt)
		clear(nonce)
		return nil, fmt.Errorf("create encryption key block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		clear(salt)
		clear(nonce)
		return nil, fmt.Errorf("create encryption key AEAD: %w", err)
	}
	sealedDEK := aead.Seal(nil, nonce, dek, envelopeAdditionalData(header, aadLabel))
	envelope := make([]byte, 0, envelopeSize)
	envelope = append(envelope, header...)
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, sealedDEK...)
	clear(salt)
	clear(nonce)
	clear(sealedDEK)
	return envelope, nil
}

func unwrapDEKWithKey(envelope, key []byte, expectedMagic, aadLabel string) ([]byte, error) {
	if err := validateCustomKey(key); err != nil {
		return nil, err
	}
	parameters, err := parseEnvelope(envelope, expectedMagic)
	if err != nil {
		return nil, err
	}
	derivedKey := argon2.IDKey(
		key,
		parameters.salt,
		parameters.argonTime,
		parameters.argonMemory,
		parameters.argonThreads,
		DEKSize,
	)
	defer clear(derivedKey)
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("create encryption key block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create encryption key AEAD: %w", err)
	}
	dek, err := aead.Open(nil, parameters.nonce, parameters.sealedDEK, envelopeAdditionalData(parameters.header, aadLabel))
	if err != nil {
		clear(dek)
		return nil, ErrAuthentication
	}
	if !validDEK(dek) {
		clear(dek)
		return nil, ErrInvalidDEK
	}
	return dek, nil
}

func WrapDEKWithCustomKey(dek, customKey []byte) ([]byte, error) {
	return wrapDEKWithKey(dek, customKey, customEnvelopeMagic, customKeyAADLabel, rand.Reader)
}

func UnwrapDEKWithCustomKey(envelope, customKey []byte) ([]byte, error) {
	return unwrapDEKWithKey(envelope, customKey, customEnvelopeMagic, customKeyAADLabel)
}

func encodeEnvelopeHeader(magic string) []byte {
	header := make([]byte, envelopeHeaderSize)
	offset := 0
	copy(header[offset:], magic)
	offset += len(magic)
	header[offset] = envelopeVersion
	offset++
	header[offset] = envelopeKDFArgon2id
	offset++
	header[offset] = envelopeCipherAESGCM
	offset++
	header[offset] = envelopeArgonVersion
	offset++
	binary.BigEndian.PutUint32(header[offset:], envelopeArgonTime)
	offset += 4
	binary.BigEndian.PutUint32(header[offset:], envelopeArgonMemory)
	offset += 4
	header[offset] = envelopeArgonThreads
	offset++
	header[offset] = envelopeSaltSize
	offset++
	header[offset] = envelopeNonceSize
	return header
}

func parseEnvelope(raw []byte, expectedMagic string) (envelopeParameters, error) {
	if len(raw) != maxEnvelopeSize || len(expectedMagic) != len(customEnvelopeMagic) {
		return envelopeParameters{}, ErrInvalidEnvelope
	}
	header := raw[:envelopeHeaderSize]
	if string(header[:len(customEnvelopeMagic)]) != expectedMagic {
		return envelopeParameters{}, ErrInvalidEnvelope
	}
	if header[len(customEnvelopeMagic)] != envelopeVersion ||
		header[len(customEnvelopeMagic)+1] != envelopeKDFArgon2id ||
		header[len(customEnvelopeMagic)+2] != envelopeCipherAESGCM {
		return envelopeParameters{}, ErrUnsupportedEnvelope
	}
	offset := len(customEnvelopeMagic) + 3
	if header[offset] != envelopeArgonVersion {
		return envelopeParameters{}, ErrUnsupportedEnvelope
	}
	offset++
	argonTime := binary.BigEndian.Uint32(header[offset:])
	offset += 4
	argonMemory := binary.BigEndian.Uint32(header[offset:])
	offset += 4
	argonThreads := header[offset]
	offset++
	saltSize := header[offset]
	offset++
	nonceSize := header[offset]
	if argonTime != envelopeArgonTime || argonMemory != envelopeArgonMemory ||
		argonThreads != envelopeArgonThreads || saltSize != envelopeSaltSize || nonceSize != envelopeNonceSize {
		return envelopeParameters{}, ErrUnsupportedEnvelope
	}
	saltStart := envelopeHeaderSize
	saltEnd := saltStart + int(saltSize)
	nonceEnd := saltEnd + int(nonceSize)
	if nonceEnd+envelopeSealedDEKSize != len(raw) {
		return envelopeParameters{}, ErrInvalidEnvelope
	}
	return envelopeParameters{
		magic:        expectedMagic,
		header:       header,
		salt:         raw[saltStart:saltEnd],
		nonce:        raw[saltEnd:nonceEnd],
		sealedDEK:    raw[nonceEnd:],
		argonTime:    argonTime,
		argonMemory:  argonMemory,
		argonThreads: argonThreads,
	}, nil
}

func envelopeAdditionalData(header []byte, label string) []byte {
	additionalData := make([]byte, 0, len(label)+len(header))
	additionalData = append(additionalData, label...)
	return append(additionalData, header...)
}
