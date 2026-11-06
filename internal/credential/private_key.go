package credential

import (
	"bytes"
	"encoding/pem"
	"errors"

	"changeme/internal/model"
	"github.com/kayrus/putty"
	xssh "golang.org/x/crypto/ssh"
)

func normalizePrivateKey(secret, passphrase string) ([]byte, string, error) {
	if len(secret) == 0 || len(secret) > model.MaxCredentialSecretSize || len(passphrase) > model.MaxCredentialSecretSize {
		return nil, "", ErrInvalidCredentialSecret
	}
	key := bytes.TrimSpace([]byte(secret))
	phrase := []byte(passphrase)
	defer clear(phrase)
	if bytes.HasPrefix(key, []byte("PuTTY-User-Key-File-")) {
		converted, err := convertPuttyKey(key, phrase)
		clear(key)
		if err != nil {
			return nil, "", err
		}
		key = converted
	}
	signer, err := xssh.ParsePrivateKey(key)
	var missing *xssh.PassphraseMissingError
	if errors.As(err, &missing) && len(phrase) > 0 {
		signer, err = xssh.ParsePrivateKeyWithPassphrase(key, phrase)
	}
	if err != nil {
		clear(key)
		return nil, "", ErrInvalidCredentialSecret
	}
	publicKey := string(bytes.TrimSpace(xssh.MarshalAuthorizedKey(signer.PublicKey())))
	if len(phrase) > 0 {
		raw, rawErr := xssh.ParseRawPrivateKey(key)
		if rawErr != nil {

			return key, publicKey, nil
		}
		block, marshalErr := xssh.MarshalPrivateKeyWithPassphrase(raw, "", phrase)
		if marshalErr != nil {
			clear(key)
			return nil, "", ErrInvalidCredentialSecret
		}
		clear(key)
		key = pem.EncodeToMemory(block)
		clear(block.Bytes)
	}
	return key, publicKey, nil
}

func convertPuttyKey(key, phrase []byte) ([]byte, error) {
	ppk, err := putty.New(key)
	if err != nil {
		return nil, ErrInvalidCredentialSecret
	}
	defer clear(ppk.PrivateKey)

	if ppk.Version == 3 && ppk.Encryption != "none" && (ppk.Argon2Memory < 8 || ppk.Argon2Memory > 256*1024 || ppk.Argon2Passes < 1 || ppk.Argon2Passes > 32 || ppk.Argon2Parallelism < 1 || ppk.Argon2Parallelism > 16 || uint64(ppk.Argon2Memory)*uint64(ppk.Argon2Passes) > 2*1024*1024) {
		return nil, ErrInvalidCredentialSecret
	}
	password := phrase
	if ppk.Encryption == "none" {
		password = nil
	}
	raw, err := ppk.ParseRawPrivateKey(password)
	if err != nil {
		return nil, ErrInvalidCredentialSecret
	}
	var block *pem.Block
	if len(phrase) > 0 {
		block, err = xssh.MarshalPrivateKeyWithPassphrase(raw, "", phrase)
	} else {
		block, err = xssh.MarshalPrivateKey(raw, "")
	}
	if err != nil {
		return nil, ErrInvalidCredentialSecret
	}
	defer clear(block.Bytes)
	return pem.EncodeToMemory(block), nil
}
