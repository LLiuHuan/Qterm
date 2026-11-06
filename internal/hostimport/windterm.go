package hostimport

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha3"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	windTermPBKDF2Iterations = 100_000
	windTermDerivedLength    = 48
	windTermKeyLength        = 32
	windTermIVLength         = aes.BlockSize
)

type windTermCrypto struct {
	key [windTermKeyLength]byte
	iv  [windTermIVLength]byte
}

func DefaultWindTermPath() string {
	return ""
}

func ParseWindTerm(path string) (Result, error) {
	return ParseWindTermWithMasterPassword(path, "")
}

func ParseWindTermWithMasterPassword(path string, masterPassword string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultWindTermPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read windterm sessions: %w", err)
	}
	crypto, cryptoErr := loadWindTermCrypto(path, masterPassword)
	if cryptoErr != nil {
		return Result{}, cryptoErr
	}
	return parseWindTermSessions(raw, crypto)
}

func loadWindTermCrypto(sessionsPath string, masterPassword string) (*windTermCrypto, error) {
	profileDir := filepath.Dir(filepath.Dir(sessionsPath))
	if profileDir == "" || profileDir == "." || profileDir == string(filepath.Separator) {
		return nil, nil
	}
	configPath := filepath.Join(profileDir, "user.config")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read windterm user.config: %w", err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse windterm user.config: %w", err)
	}
	fingerprint := windTermStringField(config, "application.fingerprint")
	if fingerprint == "" {
		return nil, fmt.Errorf("windterm user.config is missing application.fingerprint")
	}
	if windTermBoolField(config, "application.masterPassword") && masterPassword == "" {
		return nil, fmt.Errorf("%w: WindTerm master password is required", ErrImportLocked)
	}
	derived, err := deriveWindTermCrypto(fingerprint, masterPassword)
	if err != nil {
		return nil, err
	}
	return derived, nil
}

func deriveWindTermCrypto(fingerprint string, masterPassword string) (*windTermCrypto, error) {
	material, err := pbkdf2.Key(sha3.New512, masterPassword, []byte(fingerprint), windTermPBKDF2Iterations, windTermDerivedLength)
	if err != nil {
		return nil, fmt.Errorf("derive windterm key: %w", err)
	}
	crypto := &windTermCrypto{}
	copy(crypto.key[:], material[:windTermKeyLength])
	copy(crypto.iv[:], material[windTermKeyLength:])
	return crypto, nil
}

func parseWindTermSessions(raw []byte, crypto *windTermCrypto) (Result, error) {
	var sessions []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sessions); err != nil {
		return Result{}, fmt.Errorf("parse windterm sessions: %w", err)
	}
	result := Result{Entries: make([]Entry, 0, len(sessions))}
	for index, session := range sessions {

		if !strings.EqualFold(windTermStringField(session, "session.protocol"), "SSH") {
			continue
		}
		target := windTermStringField(session, "session.target")
		host, targetUser := parseWindTermTarget(target)
		if host == "" {
			result.Skipped++
			continue
		}

		port := 22
		if rawPort := session["session.port"]; len(rawPort) > 0 {
			parsed, ok := parseWindTermPort(rawPort)
			if !ok {
				result.Skipped++
				continue
			}
			port = parsed
		}
		autoLogin, loginErr := parseWindTermAutoLogin(windTermStringField(session, "session.autoLogin"), crypto)
		if loginErr != nil {
			return Result{}, loginErr
		}
		username := targetUser
		if value := windTermStringField(autoLogin, "session.user"); value != "" {
			username = value
		}
		password := ""
		if windTermBoolField(autoLogin, "PasswordEnabled") {
			password = windTermStringField(autoLogin, "Password")
		}
		label := windTermStringField(session, "session.label")
		if label == "" {
			label = host
		}
		result.Entries = append(result.Entries, Entry{
			Alias:     label,
			HostName:  host,
			User:      username,
			Port:      port,
			Group:     joinGroupSegments(strings.Split(windTermStringField(session, "session.group"), ">")),
			Password:  password,
			Line:      index + 1,
			Duplicate: false,
		})
	}
	return result, nil
}

func parseWindTermTarget(target string) (string, string) {
	target = strings.TrimSpace(target)
	if at := strings.LastIndex(target, "@"); at >= 0 {
		username := target[:at]
		host := target[at+1:]
		if username != "" && host != "" {
			return host, username
		}
	}
	return target, "root"
}

func parseWindTermPort(raw json.RawMessage) (int, bool) {
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, false
	}
	parsed, err := strconv.ParseUint(number.String(), 10, 64)
	if err != nil || parsed == 0 || parsed > 65535 {
		return 0, false
	}
	return int(parsed), true
}

func parseWindTermAutoLogin(raw string, crypto *windTermCrypto) (map[string]json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if json.Valid([]byte(raw)) {
		plain := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(raw), &plain); err != nil {

			return nil, fmt.Errorf("windterm session.autoLogin must decode to a JSON object")
		}
		return plain, nil
	}
	if crypto == nil {
		return nil, fmt.Errorf("%w: WindTerm autoLogin is encrypted but user.config was not found", ErrImportLocked)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: decode WindTerm autoLogin data", ErrImportLocked)
	}
	plaintext, err := decryptWindTermAutoLogin(ciphertext, crypto)
	if err != nil {

		return nil, fmt.Errorf("%w: decrypt WindTerm autoLogin data", ErrImportLocked)
	}
	decoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return nil, fmt.Errorf("%w: parse decrypted WindTerm autoLogin JSON", ErrImportLocked)
	}
	return decoded, nil
}

func decryptWindTermAutoLogin(ciphertext []byte, crypto *windTermCrypto) ([]byte, error) {
	if len(ciphertext) == 0 || len(ciphertext)%windTermIVLength != 0 {
		return nil, fmt.Errorf("invalid windterm autoLogin ciphertext length")
	}
	block, err := aes.NewCipher(crypto.key[:])
	if err != nil {
		return nil, fmt.Errorf("init windterm cipher: %w", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, crypto.iv[:]).CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext, windTermIVLength)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data length")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid padding size")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}

func windTermStringField(fields map[string]json.RawMessage, key string) string {
	raw, ok := fields[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func windTermBoolField(fields map[string]json.RawMessage, key string) bool {
	raw, ok := fields[key]
	if !ok {
		return false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return value
}
