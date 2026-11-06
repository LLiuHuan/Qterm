package credential

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"changeme/internal/model"
	"github.com/kayrus/putty"
	xssh "golang.org/x/crypto/ssh"
)

type GeneratePrivateKeyInput struct {
	Algorithm  string `json:"algorithm"`
	Curve      string `json:"curve,omitempty"`
	Bits       int    `json:"bits,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	Username   string `json:"username,omitempty"`
	Name       string `json:"name,omitempty"`
}

type GeneratedPrivateKey struct {
	Algorithm      string `json:"algorithm"`
	PrivateKey     string `json:"privateKey"`
	PublicKey      string `json:"publicKey"`
	InstallCommand string `json:"installCommand"`
}

type LocalPrivateKey struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	DisplayPath string `json:"displayPath"`
	Algorithm   string `json:"algorithm"`
}

type ImportLocalPrivateKeyInput struct {
	Token      string `json:"token"`
	Name       string `json:"name"`
	Username   string `json:"username,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

func (s *CredentialService) GeneratePrivateKey(ctx context.Context, input GeneratePrivateKeyInput) (GeneratedPrivateKey, error) {
	if err := contextError(ctx); err != nil {
		return GeneratedPrivateKey{}, err
	}
	algorithm := strings.ToLower(strings.TrimSpace(input.Algorithm))
	var privateKey any
	switch algorithm {
	case "ed25519":
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return GeneratedPrivateKey{}, err
		}
		privateKey = key
	case "ecdsa":
		curve := elliptic.P256()
		switch strings.ToLower(strings.TrimSpace(input.Curve)) {
		case "p-384":
			curve = elliptic.P384()
		case "p-521":
			curve = elliptic.P521()
		case "", "p-256":
		default:
			return GeneratedPrivateKey{}, errors.New("unsupported ECDSA curve")
		}
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return GeneratedPrivateKey{}, err
		}
		privateKey = key
	case "rsa":
		bits := input.Bits
		if bits == 0 {
			bits = 4096
		}
		if bits != 2048 && bits != 4096 {
			return GeneratedPrivateKey{}, errors.New("unsupported RSA key size")
		}
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			return GeneratedPrivateKey{}, err
		}
		privateKey = key
	default:
		return GeneratedPrivateKey{}, errors.New("unsupported private key algorithm")
	}
	if err := contextError(ctx); err != nil {
		return GeneratedPrivateKey{}, err
	}
	var block *pem.Block
	var err error
	if input.Passphrase == "" {
		block, err = xssh.MarshalPrivateKey(privateKey, "")
	} else {
		block, err = xssh.MarshalPrivateKeyWithPassphrase(privateKey, "", []byte(input.Passphrase))
	}
	if err != nil {
		return GeneratedPrivateKey{}, fmt.Errorf("marshal private key: %w", err)
	}
	signer, err := xssh.NewSignerFromKey(privateKey)
	if err != nil {
		return GeneratedPrivateKey{}, fmt.Errorf("create public key: %w", err)
	}
	publicKey := strings.TrimSpace(string(xssh.MarshalAuthorizedKey(signer.PublicKey())))
	return GeneratedPrivateKey{
		Algorithm:      algorithm,
		PrivateKey:     string(pem.EncodeToMemory(block)),
		PublicKey:      publicKey,
		InstallCommand: "mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%s\\n' '" + publicKey + "' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys",
	}, nil
}

func (s *CredentialService) ListLocalPrivateKeys(_ context.Context) ([]LocalPrivateKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []LocalPrivateKey{}, nil
		}
		return nil, err
	}
	result := make([]LocalPrivateKey, 0)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") || isKnownSSHMetadata(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > model.MaxCredentialSecretSize {
			continue
		}
		path := filepath.Join(sshDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		algorithm := privateKeyAlgorithm(content)
		clear(content)
		if algorithm == "" {
			continue
		}
		token, err := randomLocalKeyToken()
		if err != nil {
			return nil, err
		}
		s.localKeyMu.Lock()
		s.localKeys[token] = path
		s.localKeyMu.Unlock()
		result = append(result, LocalPrivateKey{Token: token, Name: entry.Name(), DisplayPath: filepath.Join("~", ".ssh", entry.Name()), Algorithm: algorithm})
	}
	return result, nil
}

func (s *CredentialService) ImportLocalPrivateKey(ctx context.Context, input ImportLocalPrivateKeyInput) (model.Credential, error) {
	s.localKeyMu.Lock()
	path := s.localKeys[strings.TrimSpace(input.Token)]
	s.localKeyMu.Unlock()
	if path == "" {
		return model.Credential{}, errors.New("local private key token is expired")
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		return model.Credential{}, errors.New("local private key is unavailable")
	}
	defer clear(secret)
	return s.CreateCredential(ctx, CreateCredentialInput{
		Name: input.Name, Kind: model.CredentialKindPrivateKey, Secret: string(secret),
		Passphrase: input.Passphrase, Username: input.Username,
	})
}

func privateKeyAlgorithm(value []byte) string {
	key, err := xssh.ParseRawPrivateKey(value)
	if err == nil {
		return privateKeyAlgorithmValue(key)
	}
	if bytes.HasPrefix(bytes.TrimSpace(value), []byte("PuTTY-User-Key-")) {
		if ppk, ppkErr := putty.New(value); ppkErr == nil {
			return strings.TrimPrefix(ppk.Algo, "ssh-")
		}
	}
	if bytes.HasPrefix(bytes.TrimSpace(value), []byte("-----BEGIN OPENSSH PRIVATE KEY-----")) {
		return "encrypted"
	}
	if block, _ := pem.Decode(value); block != nil && strings.Contains(strings.ToLower(block.Type), "rsa") {
		return "rsa"
	}
	return ""
}

func privateKeyAlgorithmValue(value any) string {

	switch value.(type) {
	case ed25519.PrivateKey, *ed25519.PrivateKey:
		return "ed25519"
	case *ecdsa.PrivateKey:
		return "ecdsa"
	case *rsa.PrivateKey:
		return "rsa"
	default:
		return ""
	}
}

func isKnownSSHMetadata(name string) bool {
	switch strings.ToLower(name) {
	case "config", "known_hosts", "known_hosts2", "authorized_keys", "environment":
		return true
	default:
		return strings.HasSuffix(name, ".pub")
	}
}

func randomLocalKeyToken() (string, error) {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
