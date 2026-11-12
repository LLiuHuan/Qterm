package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "Qterm"
	requestTimeout = 90 * time.Second
	maxReplyBytes  = 1 << 20
	anthropicVer   = "2023-06-01"
)

var ErrNotConfigured = errors.New("ai provider is not configured")

func anthropicAPIBase(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return ""
	}
	if index := strings.Index(base, "://"); index >= 0 && !strings.Contains(base[index+3:], "/") {
		return base + "/v1"
	}
	return base
}

type StatusError struct {
	Status int

	Detail string
}

func (e *StatusError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("ai provider returned status %d: %s", e.Status, e.Detail)
	}
	return fmt.Sprintf("ai provider returned status %d", e.Status)
}

func upstreamDetail(data []byte, secrets ...string) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ""
	}
	detail := strings.TrimSpace(parsed.Error.Message)
	if detail == "" {
		detail = strings.TrimSpace(parsed.Message)
	}
	detail = redactSecrets(detail, secrets...)
	detail = strings.Join(strings.Fields(detail), " ")
	if len(detail) > 200 {
		detail = detail[:200]
	}
	return detail
}

func redactSecrets(value string, secrets ...string) string {
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		value = strings.ReplaceAll(value, secret, "[redacted]")

		normalized := strings.Join(strings.Fields(secret), " ")
		if normalized != secret {
			value = strings.ReplaceAll(value, normalized, "[redacted]")
		}
	}
	return value
}

type ProviderType string

const (
	ProviderTypeOpenAI         ProviderType = "openai"
	ProviderTypeOpenAIResponse ProviderType = "openai-response"
	ProviderTypeAnthropic      ProviderType = "anthropic"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	Images []string `json:"images,omitempty"`
}

type Provider struct {
	ID      string       `json:"id"`
	Type    ProviderType `json:"type"`
	BaseURL string       `json:"baseUrl"`
	Model   string       `json:"model"`
}

type ChatRequest struct {
	Provider Provider  `json:"provider"`
	Messages []Message `json:"messages"`

	JSONMode bool `json:"jsonMode,omitempty"`

	Context ContextBreakdown `json:"-"`
}

type Service struct {
	client *http.Client

	keyLookup func(providerID string) (string, error)
}

func NewService() *Service {
	return &Service{
		client:    &http.Client{Timeout: requestTimeout},
		keyLookup: readKeyringKey,
	}
}

func readKeyringKey(providerID string) (string, error) {
	return keyring.Get(keyringService, keyAccount(providerID))
}

func (s *Service) providerKey(providerID string) (string, error) {
	if s.keyLookup != nil {
		return s.keyLookup(providerID)
	}
	return readKeyringKey(providerID)
}

func keyAccount(providerID string) string {
	return "ai.key." + strings.TrimSpace(providerID)
}

func (s *Service) SetKey(providerID, key string) error {
	id := strings.TrimSpace(providerID)
	if id == "" {
		return errors.New("ai provider id is required")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		err := keyring.Delete(keyringService, keyAccount(id))
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("clear ai api key: %w", err)
		}
		return nil
	}
	if err := keyring.Set(keyringService, keyAccount(id), key); err != nil {
		return fmt.Errorf("store ai api key: %w", err)
	}
	return nil
}

func (s *Service) ClearKey(providerID string) error {
	return s.SetKey(providerID, "")
}

func (s *Service) KeyConfigured(providerID string) bool {
	id := strings.TrimSpace(providerID)
	if id == "" {
		return false
	}
	value, err := keyring.Get(keyringService, keyAccount(id))
	return err == nil && strings.TrimSpace(value) != ""
}

func (s *Service) Chat(ctx context.Context, request ChatRequest) (string, error) {
	if s == nil {
		return "", errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	provider := request.Provider
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	model := strings.TrimSpace(provider.Model)
	if baseURL == "" || model == "" {
		return "", ErrNotConfigured
	}
	key, err := s.providerKey(provider.ID)
	if err != nil || strings.TrimSpace(key) == "" {
		return "", ErrNotConfigured
	}
	return s.chatWithKey(ctx, request, key)
}

func (s *Service) ChatWithTemporaryKey(ctx context.Context, request ChatRequest, draftKey string) (string, error) {
	if strings.TrimSpace(draftKey) == "" {
		return s.Chat(ctx, request)
	}
	if s == nil {
		return "", errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	provider := request.Provider
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	model := strings.TrimSpace(provider.Model)
	if baseURL == "" || model == "" {
		return "", ErrNotConfigured
	}
	return s.chatWithKey(ctx, request, strings.TrimSpace(draftKey))
}

func (s *Service) chatWithKey(ctx context.Context, request ChatRequest, key string) (string, error) {
	if s == nil {
		return "", errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	provider := request.Provider
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	model := strings.TrimSpace(provider.Model)
	if baseURL == "" || model == "" || strings.TrimSpace(key) == "" {
		return "", ErrNotConfigured
	}
	if _, err := countMessageImages(request.Messages); err != nil {
		return "", err
	}
	switch provider.Type {
	case ProviderTypeOpenAI:
		return s.chatOpenAI(ctx, baseURL, model, key, request.Messages)
	case ProviderTypeOpenAIResponse:
		return s.chatOpenAIResponse(ctx, baseURL, model, key, request.Messages)
	case ProviderTypeAnthropic:
		return s.chatAnthropic(ctx, baseURL, model, key, request.Messages)
	default:
		return "", ErrNotConfigured
	}
}

func (s *Service) chatOpenAI(
	ctx context.Context,
	baseURL, model, key string,
	messages []Message,
) (string, error) {
	converted, err := openAIChatMessages(messages)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"model": model, "messages": converted, "stream": false}
	data, err := s.postJSON(ctx, baseURL+"/chat/completions", key, payload, nil)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("ai provider returned no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (s *Service) chatOpenAIResponse(
	ctx context.Context,
	baseURL, model, key string,
	messages []Message,
) (string, error) {

	instructions, input, err := splitResponsesInput(messages)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"model": model, "input": input, "stream": false}
	if instructions != "" {
		payload["instructions"] = instructions
	}
	data, err := s.postJSON(ctx, baseURL+"/responses", key, payload, nil)
	if err != nil {
		return "", err
	}
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText), nil
	}
	var builder strings.Builder
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" || content.Type == "text" {
				builder.WriteString(content.Text)
			}
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", errors.New("ai provider returned no output text")
	}
	return text, nil
}

func (s *Service) chatAnthropic(
	ctx context.Context,
	baseURL, model, key string,
	messages []Message,
) (string, error) {
	system, converted, err := splitAnthropicMessages(messages)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"model": model, "max_tokens": 4096, "messages": converted}
	if system != "" {
		payload["system"] = system
	}
	data, err := s.postJSON(ctx, anthropicAPIBase(baseURL)+"/messages", key, payload, func(request *http.Request) {

		request.Header.Del("Authorization")
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", anthropicVer)
	})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	var builder strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			builder.WriteString(block.Text)
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", errors.New("ai provider returned no text")
	}
	return text, nil
}

func (s *Service) ListModels(ctx context.Context, provider Provider) ([]string, error) {
	if s == nil {
		return nil, errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" {
		return nil, ErrNotConfigured
	}
	key, err := s.providerKey(provider.ID)
	if err != nil || strings.TrimSpace(key) == "" {
		return nil, ErrNotConfigured
	}
	return s.listModelsWithKey(ctx, provider, key)
}

func (s *Service) ListModelsWithTemporaryKey(ctx context.Context, provider Provider, draftKey string) ([]string, error) {
	if strings.TrimSpace(draftKey) == "" {
		return s.ListModels(ctx, provider)
	}
	if s == nil {
		return nil, errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" {
		return nil, ErrNotConfigured
	}
	return s.listModelsWithKey(ctx, provider, strings.TrimSpace(draftKey))
}

func (s *Service) listModelsWithKey(ctx context.Context, provider Provider, key string) ([]string, error) {
	if s == nil {
		return nil, errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(key) == "" {
		return nil, ErrNotConfigured
	}

	modelsBase := baseURL
	if provider.Type == ProviderTypeAnthropic {
		modelsBase = anthropicAPIBase(baseURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsBase+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("build ai models request: %w", err)
	}
	if provider.Type == ProviderTypeAnthropic {
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", anthropicVer)
	} else {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list ai models: %w", err)
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxReplyBytes))
	if readErr != nil {
		return nil, fmt.Errorf("read ai models: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &StatusError{Status: response.StatusCode, Detail: upstreamDetail(data, key)}
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode ai models: %w", err)
	}
	models := make([]string, 0, len(parsed.Data)+len(parsed.Models))
	for _, item := range parsed.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	for _, item := range parsed.Models {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	return models, nil
}

func (s *Service) postJSON(
	ctx context.Context,
	url, key string,
	payload any,
	customize func(*http.Request),
) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode ai request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build ai request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)
	if customize != nil {
		customize(request)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call ai provider: %w", err)
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxReplyBytes))
	if readErr != nil {
		return nil, fmt.Errorf("read ai response: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &StatusError{Status: response.StatusCode, Detail: upstreamDetail(data, key)}
	}
	return data, nil
}
