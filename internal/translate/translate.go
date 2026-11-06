package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Provider string

const (
	ProviderGoogle Provider = "google"

	ProviderMicrosoft Provider = "microsoft"

	ProviderDeepL Provider = "deepl"

	ProviderBaidu Provider = "baidu"

	ProviderAli Provider = "ali"

	ProviderYoudao Provider = "youdao"

	ProviderLibreTranslate Provider = "libretranslate"
)

const maxTextLength = 4000

const maxResponseBytes = 1 << 20

var ErrUnsupportedProvider = errors.New("translate: unsupported provider")

var (
	errGoogleUnavailable    = errors.New("translate: google is temporarily unavailable")
	errMicrosoftUnavailable = errors.New("translate: microsoft is temporarily unavailable")
)

type Result struct {
	Text             string `json:"text"`
	DetectedLanguage string `json:"detectedLanguage"`
}

type Credentials struct {
	APIKey string
	AppID  string
	AppKey string
}

type Service struct {
	client *http.Client

	microsoftClient *http.Client

	googleEndpoint         string
	googleClients5Endpoint string
}

func NewService() *Service {

	jar, _ := cookiejar.New(nil)

	transport := newTranslateTransport()
	return &Service{
		client: &http.Client{Timeout: 15 * time.Second, Transport: transport},
		microsoftClient: &http.Client{
			Timeout:   15 * time.Second,
			Jar:       jar,
			Transport: transport,

			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				if len(via) > 0 && via[len(via)-1].Method == http.MethodPost {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		googleClients5Endpoint: "https://clients5.google.com/translate_a/t",
		googleEndpoint:         "https://translate.googleapis.com/translate_a/single",
	}
}

func (s *Service) Translate(ctx context.Context, text, target string, provider Provider, endpoint string) (Result, error) {
	return s.TranslateWithCredentials(ctx, text, target, provider, endpoint, Credentials{})
}

func (s *Service) TranslateWithCredentials(ctx context.Context, text, target string, provider Provider, endpoint string, credentials Credentials) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("translate: context is nil")
	}
	source := strings.TrimSpace(text)
	if source == "" {
		return Result{}, errors.New("translate: empty text")
	}
	if len(source) > maxTextLength {
		source = source[:maxTextLength]
	}
	language := normalizeLanguage(target)
	switch provider {
	case ProviderGoogle, "":
		return s.google(ctx, source, language)
	case ProviderMicrosoft:
		return s.microsoft(ctx, source, language)
	case ProviderDeepL:
		return s.deepl(ctx, source, language, credentials)
	case ProviderBaidu:
		return s.baidu(ctx, source, language, credentials)
	case ProviderAli:
		return s.ali(ctx, source, language, credentials)
	case ProviderYoudao:
		return s.youdao(ctx, source, language, credentials)
	case ProviderLibreTranslate:
		return s.libreTranslate(ctx, source, language, endpoint)
	default:
		return Result{}, ErrUnsupportedProvider
	}
}

func Languages() []string {
	return []string{"zh-CN", "zh-TW", "en", "ja", "ko", "fr", "de", "ru", "es"}
}

func normalizeLanguage(target string) string {
	value := strings.TrimSpace(target)
	if value == "" {
		return "zh-CN"
	}
	return value
}

func (s *Service) google(ctx context.Context, text, target string) (Result, error) {
	result, err := s.googleClients5(ctx, text, target)
	if err == nil {
		return result, nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Result{}, err
	}
	result, err = s.googleLegacy(ctx, text, target)
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Result{}, err
	}
	return Result{}, errGoogleUnavailable
}

func (s *Service) googleClients5(ctx context.Context, text, target string) (Result, error) {
	query := url.Values{}
	query.Set("client", "dict-chrome-ex")
	query.Set("sl", "auto")
	query.Set("tl", target)
	query.Set("q", text)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.googleClients5Endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("User-Agent", browserUserAgent)
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var payload []any
	if err := json.Unmarshal(body, &payload); err != nil {
		return Result{}, errors.New("translate: unexpected google clients5 response")
	}
	translated, detected, ok := parseGoogleClients5(payload)
	if !ok {
		return Result{}, errors.New("translate: empty google clients5 result")
	}
	return Result{Text: translated, DetectedLanguage: detected}, nil
}

func parseGoogleClients5(payload []any) (string, string, bool) {
	if len(payload) == 0 {
		return "", "", false
	}
	stringValue := func(value any) string {
		text, _ := value.(string)
		return strings.TrimSpace(text)
	}
	if segment, ok := payload[0].([]any); ok {
		if len(segment) > 0 {
			if _, isString := segment[0].(string); isString {
				detected := ""
				if len(segment) > 1 {
					detected = stringValue(segment[1])
				}
				translated := stringValue(segment[0])
				return translated, detected, translated != ""
			}
		}
		var builder strings.Builder
		detected := ""
		for _, item := range segment {
			parts, ok := item.([]any)
			if !ok || len(parts) == 0 {
				continue
			}
			piece := stringValue(parts[0])
			if piece == "" {
				continue
			}
			builder.WriteString(piece)
			if detected == "" && len(parts) > 1 {
				detected = stringValue(parts[1])
			}
		}
		translated := strings.TrimSpace(builder.String())
		return translated, detected, translated != ""
	}
	if translated := stringValue(payload[0]); translated != "" {
		detected := ""
		if len(payload) > 1 {
			detected = stringValue(payload[1])
		}
		return translated, detected, true
	}
	return "", "", false
}

func (s *Service) googleLegacy(ctx context.Context, text, target string) (Result, error) {
	query := url.Values{}
	query.Set("client", "gtx")
	query.Set("sl", "auto")
	query.Set("tl", target)
	query.Set("dt", "t")
	query.Set("q", text)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.googleEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		return Result{}, err
	}
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var payload []any
	if err := json.Unmarshal(body, &payload); err != nil {
		return Result{}, errors.New("translate: unexpected google response")
	}
	if len(payload) == 0 {
		return Result{}, errors.New("translate: empty google response")
	}
	segments, ok := payload[0].([]any)
	if !ok {
		return Result{}, errors.New("translate: unexpected google response")
	}
	var builder strings.Builder
	for _, segment := range segments {
		parts, ok := segment.([]any)
		if !ok || len(parts) == 0 {
			continue
		}
		if piece, ok := parts[0].(string); ok {
			builder.WriteString(piece)
		}
	}
	result := builder.String()
	if strings.TrimSpace(result) == "" {
		return Result{}, errors.New("translate: empty google result")
	}
	detectedLanguage := ""
	if len(payload) > 2 {
		if value, ok := payload[2].(string); ok {
			detectedLanguage = strings.TrimSpace(value)
		}
	}
	return Result{Text: result, DetectedLanguage: detectedLanguage}, nil
}

func (s *Service) libreTranslate(ctx context.Context, text, target, endpoint string) (Result, error) {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if base == "" {
		return Result{}, errors.New("translate: libretranslate endpoint is required")
	}
	payload, err := json.Marshal(map[string]string{
		"q":      text,
		"source": "auto",
		"target": target,
		"format": "text",
	})
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/translate", bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var result struct {
		TranslatedText   string `json:"translatedText"`
		DetectedLanguage struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("translate: decode libretranslate response: %w", err)
	}
	if strings.TrimSpace(result.TranslatedText) == "" {
		return Result{}, errors.New("translate: empty libretranslate result")
	}
	return Result{
		Text:             result.TranslatedText,
		DetectedLanguage: strings.TrimSpace(result.DetectedLanguage.Language),
	}, nil
}

func (s *Service) do(request *http.Request) ([]byte, error) {
	return s.doWith(s.client, request)
}

func (s *Service) doWith(client *http.Client, request *http.Request) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		return nil, errors.New("translate: request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("translate: read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("translate: upstream response too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("translate: upstream status %d", response.StatusCode)
	}
	return body, nil
}
