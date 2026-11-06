package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Type string

const (
	TypeTOTP Type = "totp"

	TypeHOTP Type = "hotp"
)

const (
	DefaultDigits    = 6
	DefaultPeriod    = 30
	DefaultAlgorithm = "SHA1"
)

const (
	MinDigits  = 4
	MaxDigits  = 10
	MinPeriod  = 10
	MaxPeriod  = 300
	MaxCounter = 999999999
)

var (
	ErrInvalidSecret = errors.New("otp: invalid base32 secret")

	ErrUnsupportedAlgorithm = errors.New("otp: unsupported algorithm")

	ErrInvalidDigits = errors.New("otp: invalid digits")

	ErrInvalidPeriod = errors.New("otp: invalid period")

	ErrInvalidCounter = errors.New("otp: invalid counter")
)

type Config struct {
	Type      Type
	Algorithm string
	Digits    int
	Period    int
	Counter   uint64
}

func (c Config) Normalize() (Config, error) {
	result := c
	if result.Type == "" {
		result.Type = TypeTOTP
	}
	if result.Type != TypeTOTP && result.Type != TypeHOTP {
		return Config{}, fmt.Errorf("otp: unsupported type %q", c.Type)
	}
	result.Algorithm = strings.ToUpper(strings.TrimSpace(result.Algorithm))
	if result.Algorithm == "" {
		result.Algorithm = DefaultAlgorithm
	}
	if _, err := hashFor(result.Algorithm); err != nil {
		return Config{}, err
	}
	if result.Digits == 0 {
		result.Digits = DefaultDigits
	}
	if result.Digits < MinDigits || result.Digits > MaxDigits {
		return Config{}, ErrInvalidDigits
	}
	if result.Type == TypeTOTP {
		if result.Period == 0 {
			result.Period = DefaultPeriod
		}
		if result.Period < MinPeriod || result.Period > MaxPeriod {
			return Config{}, ErrInvalidPeriod
		}
	} else if result.Counter > MaxCounter {
		return Config{}, ErrInvalidCounter
	}
	return result, nil
}

func hashFor(algorithm string) (func() hash.Hash, error) {
	switch strings.ToUpper(algorithm) {
	case "SHA1":
		return sha1.New, nil
	case "SHA256":
		return sha256.New, nil
	case "SHA512":
		return sha512.New, nil
	default:
		return nil, ErrUnsupportedAlgorithm
	}
}

func DecodeSecret(secret string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(secret))
	cleaned = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "-", "").Replace(cleaned)
	cleaned = strings.TrimRight(cleaned, "=")
	if cleaned == "" {
		return nil, ErrInvalidSecret
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil || len(decoded) == 0 {
		return nil, ErrInvalidSecret
	}
	return decoded, nil
}

func EncodeSecret(secret []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

func Generate(config Config, secret []byte, at time.Time) (string, error) {
	normalized, err := config.Normalize()
	if err != nil {
		return "", err
	}
	if len(secret) == 0 {
		return "", ErrInvalidSecret
	}
	counter := normalized.Counter
	if normalized.Type == TypeTOTP {
		counter = CounterFor(at, normalized.Period)
	}
	return generateCode(normalized, secret, counter)
}

func CounterFor(at time.Time, period int) uint64 {
	if period <= 0 {
		period = DefaultPeriod
	}
	seconds := at.Unix()
	if seconds < 0 {
		return 0
	}
	return uint64(seconds / int64(period))
}

func RemainingSeconds(at time.Time, period int) int {
	if period <= 0 {
		period = DefaultPeriod
	}
	elapsed := int(at.Unix()) % period
	if elapsed < 0 {
		elapsed = 0
	}
	return period - elapsed
}

func generateCode(config Config, secret []byte, counter uint64) (string, error) {
	hashFunc, err := hashFor(config.Algorithm)
	if err != nil {
		return "", err
	}
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(hashFunc, secret)
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	truncated := (uint32(digest[offset]&0x7f) << 24) |
		(uint32(digest[offset+1]) << 16) |
		(uint32(digest[offset+2]) << 8) |
		uint32(digest[offset+3])
	modulus := uint32(1)
	for i := 0; i < config.Digits; i++ {
		modulus *= 10
	}
	return fmt.Sprintf("%0*d", config.Digits, truncated%modulus), nil
}

func ParseURI(uri string) (label string, config Config, secret string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return "", Config{}, "", fmt.Errorf("otp: invalid uri")
	}
	if !strings.EqualFold(parsed.Scheme, "otpauth") {
		return "", Config{}, "", fmt.Errorf("otp: unsupported scheme")
	}
	switch strings.ToLower(parsed.Host) {
	case "totp":
		config.Type = TypeTOTP
	case "hotp":
		config.Type = TypeHOTP
	default:
		return "", Config{}, "", fmt.Errorf("otp: unsupported type")
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	if decoded, decodeErr := url.PathUnescape(path); decodeErr == nil {
		path = decoded
	}
	label = path
	query := parsed.Query()
	secret = strings.TrimSpace(query.Get("secret"))
	if secret == "" {
		return "", Config{}, "", ErrInvalidSecret
	}
	config.Algorithm = strings.ToUpper(query.Get("algorithm"))
	config.Digits = parsePositive(query.Get("digits"), 0)
	config.Period = parsePositive(query.Get("period"), 0)
	config.Counter, _ = strconv.ParseUint(query.Get("counter"), 10, 64)
	return label, config, secret, nil
}

func parsePositive(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func BuildURI(config Config, label, secret string) string {
	normalized, err := config.Normalize()
	if err != nil {
		normalized = Config{Type: TypeTOTP, Algorithm: DefaultAlgorithm, Digits: DefaultDigits, Period: DefaultPeriod}
	}
	account := label
	issuer := ""
	if index := strings.Index(label, ":"); index > 0 {
		issuer = strings.TrimSpace(label[:index])
		account = strings.TrimSpace(label[index+1:])
	}
	query := url.Values{}
	query.Set("secret", strings.TrimRight(strings.ToUpper(secret), "="))
	if normalized.Type == TypeTOTP {
		if normalized.Algorithm != DefaultAlgorithm {
			query.Set("algorithm", normalized.Algorithm)
		}
		if normalized.Digits != DefaultDigits {
			query.Set("digits", strconv.Itoa(normalized.Digits))
		}
		if normalized.Period != DefaultPeriod {
			query.Set("period", strconv.Itoa(normalized.Period))
		}
	} else {
		query.Set("counter", strconv.FormatUint(normalized.Counter, 10))
	}
	if issuer != "" {
		query.Set("issuer", issuer)
	}
	path := url.PathEscape(account)
	if issuer != "" {
		path = url.PathEscape(issuer + ":" + account)
	}
	return fmt.Sprintf("otpauth://%s/%s?%s", normalized.Type, path, query.Encode())
}
