package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type Usage struct {
	InputTokens      *int64 `json:"inputTokens,omitempty"`
	OutputTokens     *int64 `json:"outputTokens,omitempty"`
	TotalTokens      *int64 `json:"totalTokens,omitempty"`
	CacheReadTokens  *int64 `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens *int64 `json:"cacheWriteTokens,omitempty"`
}

type StreamHandler func(delta string, reasoning string, usage *Usage) error

func (s *Service) StreamChat(ctx context.Context, request ChatRequest, handle StreamHandler) error {
	if s == nil {
		return errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	provider := request.Provider
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	model := strings.TrimSpace(provider.Model)
	if baseURL == "" || model == "" {
		return ErrNotConfigured
	}
	key, err := s.providerKey(provider.ID)
	if err != nil || strings.TrimSpace(key) == "" {
		return ErrNotConfigured
	}

	if _, err := countMessageImages(request.Messages); err != nil {
		return err
	}

	url := ""
	payload := map[string]any{}
	customize := func(*http.Request) {}
	switch provider.Type {
	case ProviderTypeOpenAI:
		messages, err := openAIChatMessages(request.Messages)
		if err != nil {
			return err
		}
		url = baseURL + "/chat/completions"
		payload = map[string]any{"model": model, "messages": messages, "stream": true}
		if request.JSONMode {

			payload["response_format"] = map[string]any{"type": "json_object"}
		}

		if streamOptionsEnabled(provider) {
			payload["stream_options"] = map[string]any{"include_usage": true}
		}
	case ProviderTypeOpenAIResponse:
		instructions, input, err := splitResponsesInput(request.Messages)
		if err != nil {
			return err
		}
		url = baseURL + "/responses"
		payload = map[string]any{"model": model, "input": input, "stream": true}
		if instructions != "" {
			payload["instructions"] = instructions
		}
	case ProviderTypeAnthropic:
		system, converted, err := splitAnthropicMessages(request.Messages)
		if err != nil {
			return err
		}
		url = anthropicAPIBase(baseURL) + "/messages"
		payload = map[string]any{"model": model, "max_tokens": 4096, "messages": converted, "stream": true}
		if system != "" {
			payload["system"] = system
		}
		customize = func(request *http.Request) {
			request.Header.Del("Authorization")
			request.Header.Set("x-api-key", key)
			request.Header.Set("anthropic-version", anthropicVer)
		}
	default:
		return ErrNotConfigured
	}

	response, err := s.streamRequest(ctx, provider, payload, func(body []byte) (*http.Request, error) {
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Accept", "text/event-stream")
		httpRequest.Header.Set("Authorization", "Bearer "+key)
		customize(httpRequest)
		return httpRequest, nil
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return readSSE(response.Body, provider.Type, handle)
}

var (
	streamOptionsRejected sync.Map

	streamOptionsRetried sync.Map
)

func streamOptionsKey(provider Provider) string {
	return string(provider.Type) + "|" + strings.TrimSpace(provider.BaseURL) + "|" + strings.TrimSpace(provider.Model)
}

func streamOptionsEnabled(provider Provider) bool {
	_, rejected := streamOptionsRejected.Load(streamOptionsKey(provider))
	return !rejected
}

func streamOptionsRetryAllowed(provider Provider) bool {
	_, retried := streamOptionsRetried.Load(streamOptionsKey(provider))
	return !retried
}

func (s *Service) streamRequest(
	ctx context.Context,
	provider Provider,
	payload map[string]any,
	build func(body []byte) (*http.Request, error),
) (*http.Response, error) {
	includeUsage := provider.Type == ProviderTypeOpenAI && streamOptionsEnabled(provider)
	response, err := s.streamRequestOnce(ctx, payload, build)
	if err != nil {
		return nil, err
	}
	if isStreamSuccess(response.StatusCode) {
		return response, nil
	}
	first := readStreamStatusError(response)
	if !includeUsage || !streamOptionsRetryAllowed(provider) || !isStreamOptionsRejection(first.Status, first.Detail) {
		return nil, first
	}

	streamOptionsRetried.Store(streamOptionsKey(provider), struct{}{})
	delete(payload, "stream_options")
	retryResponse, retryErr := s.streamRequestOnce(ctx, payload, build)
	if retryErr != nil {
		return nil, retryErr
	}
	if isStreamSuccess(retryResponse.StatusCode) {

		streamOptionsRejected.Store(streamOptionsKey(provider), struct{}{})
		return retryResponse, nil
	}

	_ = readStreamStatusError(retryResponse)
	return nil, first
}

func (s *Service) streamRequestOnce(
	ctx context.Context,
	payload map[string]any,
	build func(body []byte) (*http.Request, error),
) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode ai request: %w", err)
	}
	request, err := build(body)
	if err != nil {
		return nil, fmt.Errorf("build ai request: %w", err)
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call ai provider: %w", err)
	}
	return response, nil
}

func isStreamSuccess(status int) bool {
	return status >= 200 && status < 300
}

func readStreamStatusError(response *http.Response) *StatusError {
	data, _ := io.ReadAll(io.LimitReader(response.Body, maxReplyBytes))
	response.Body.Close()
	return &StatusError{Status: response.StatusCode, Detail: upstreamDetail(data)}
}

func isStreamOptionsRejection(status int, detail string) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusNotImplemented:
	default:
		return false
	}
	text := strings.ToLower(detail)
	if strings.Contains(text, "stream_options") || strings.Contains(text, "stream options") {
		return true
	}
	for _, phrase := range []string{
		"unknown parameter",
		"unrecognized request argument",
		"unrecognized field",
		"unexpected keyword argument",
		"extra inputs are not permitted",
		"unknown field",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func readSSE(body io.Reader, providerType ProviderType, handle StreamHandler) error {
	reader := bufio.NewReaderSize(body, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data != "" && data != "[DONE]" {
					delta, reasoning, usage, ok := extractDelta(data, providerType)
					if ok && (delta != "" || reasoning != "" || usage != nil) {
						if handlerErr := handle(delta, reasoning, usage); handlerErr != nil {
							return handlerErr
						}
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read ai stream: %w", err)
		}
	}
}

func extractDelta(data string, providerType ProviderType) (string, string, *Usage, bool) {
	switch providerType {
	case ProviderTypeOpenAI:
		var event struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *openAIUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return "", "", nil, false
		}
		usage := event.Usage.toUsage()
		if len(event.Choices) == 0 {
			return "", "", usage, usage != nil
		}
		return event.Choices[0].Delta.Content, event.Choices[0].Delta.ReasoningContent, usage, true
	case ProviderTypeOpenAIResponse:
		var event struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response *struct {
				Usage *responsesUsage `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return "", "", nil, false
		}
		switch event.Type {
		case "response.output_text.delta":
			return event.Delta, "", nil, true
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			return "", event.Delta, nil, true
		case "response.completed":
			if event.Response == nil {
				return "", "", nil, false
			}
			usage := event.Response.Usage.toUsage()
			return "", "", usage, usage != nil
		default:
			return "", "", nil, false
		}
	case ProviderTypeAnthropic:
		var event struct {
			Type    string          `json:"type"`
			Usage   *anthropicUsage `json:"usage"`
			Message *struct {
				Usage *anthropicUsage `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return "", "", nil, false
		}
		switch event.Type {
		case "message_start":
			if event.Message == nil {
				return "", "", nil, false
			}
			usage := event.Message.Usage.toUsage()
			return "", "", usage, usage != nil
		case "message_delta":
			usage := event.Usage.toUsage()
			return "", "", usage, usage != nil
		case "content_block_delta":

		default:
			return "", "", nil, false
		}
		switch event.Delta.Type {
		case "thinking_delta":
			return "", event.Delta.Thinking, nil, true
		default:
			return event.Delta.Text, "", nil, true
		}
	default:
		return "", "", nil, false
	}
}

type openAIUsage struct {
	PromptTokens         *int64 `json:"prompt_tokens"`
	CompletionTokens     *int64 `json:"completion_tokens"`
	TotalTokens          *int64 `json:"total_tokens"`
	PromptCacheHitTokens *int64 `json:"prompt_cache_hit_tokens"`
	PromptTokensDetails  *struct {
		CachedTokens *int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

func (value *openAIUsage) toUsage() *Usage {
	if value == nil {
		return nil
	}
	cacheRead := value.PromptCacheHitTokens
	if cacheRead == nil && value.PromptTokensDetails != nil {
		cacheRead = value.PromptTokensDetails.CachedTokens
	}
	usage := &Usage{
		InputTokens:     value.PromptTokens,
		OutputTokens:    value.CompletionTokens,
		TotalTokens:     value.TotalTokens,
		CacheReadTokens: cacheRead,
	}
	return usageOrNil(usage)
}

type responsesUsage struct {
	InputTokens        *int64 `json:"input_tokens"`
	OutputTokens       *int64 `json:"output_tokens"`
	TotalTokens        *int64 `json:"total_tokens"`
	InputTokensDetails *struct {
		CachedTokens *int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func (value *responsesUsage) toUsage() *Usage {
	if value == nil {
		return nil
	}
	var cacheRead *int64
	if value.InputTokensDetails != nil {
		cacheRead = value.InputTokensDetails.CachedTokens
	}
	return usageOrNil(&Usage{
		InputTokens:     value.InputTokens,
		OutputTokens:    value.OutputTokens,
		TotalTokens:     value.TotalTokens,
		CacheReadTokens: cacheRead,
	})
}

type anthropicUsage struct {
	InputTokens              *int64 `json:"input_tokens"`
	OutputTokens             *int64 `json:"output_tokens"`
	CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64 `json:"cache_read_input_tokens"`
}

func (value *anthropicUsage) toUsage() *Usage {
	if value == nil {
		return nil
	}
	return usageOrNil(&Usage{
		InputTokens:      value.InputTokens,
		OutputTokens:     value.OutputTokens,
		CacheReadTokens:  value.CacheReadInputTokens,
		CacheWriteTokens: value.CacheCreationInputTokens,
	})
}

func usageOrNil(value *Usage) *Usage {
	if value == nil || (value.InputTokens == nil && value.OutputTokens == nil && value.TotalTokens == nil && value.CacheReadTokens == nil && value.CacheWriteTokens == nil) {
		return nil
	}
	return value
}
