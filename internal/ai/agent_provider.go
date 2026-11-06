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

type AgentDelta struct {
	Content   string
	Reasoning string
	Usage     *Usage
}

type AgentTurn struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Usage     *Usage
}

func (s *Service) StreamAgentTurn(
	ctx context.Context,
	provider Provider,
	messages []AgentMessage,
	tools []ToolSpec,
	handle func(AgentDelta) error,
) (AgentTurn, error) {
	if s == nil {
		return AgentTurn{}, errors.New("ai service is not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	model := strings.TrimSpace(provider.Model)
	if baseURL == "" || model == "" || provider.ID == "" {
		return AgentTurn{}, ErrNotConfigured
	}
	if _, rejected := nativeToolsRejected.Load(providerToolsKey(provider)); rejected {
		return AgentTurn{}, ErrToolsUnsupported
	}
	key, err := s.providerKey(provider.ID)
	if err != nil || strings.TrimSpace(key) == "" {
		return AgentTurn{}, ErrNotConfigured
	}

	url := ""
	payload := map[string]any{}
	customize := func(*http.Request) {}
	switch provider.Type {
	case ProviderTypeOpenAI:
		url = baseURL + "/chat/completions"
		payload = map[string]any{
			"model":    model,
			"messages": openAIAgentMessages(messages),
			"tools":    openAITools(tools),

			"tool_choice": "required",
			"stream":      true,
		}

		if streamOptionsEnabled(provider) {
			payload["stream_options"] = map[string]any{"include_usage": true}
		}
	case ProviderTypeAnthropic:
		url = anthropicAPIBase(baseURL) + "/messages"
		system, converted := anthropicAgentMessages(messages)
		payload = map[string]any{
			"model":      model,
			"max_tokens": 4096,
			"messages":   converted,
			"tools":      anthropicTools(tools),

			"tool_choice": map[string]any{"type": "any"},
			"stream":      true,
		}
		if system != "" {
			payload["system"] = system
		}
		customize = func(request *http.Request) {
			request.Header.Del("Authorization")
			request.Header.Set("x-api-key", key)
			request.Header.Set("anthropic-version", anthropicVer)
		}
	default:
		return AgentTurn{}, fmt.Errorf("%w: native tools are not supported for this provider type", ErrToolsUnsupported)
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

		markNativeToolsRejected(provider, err)
		return AgentTurn{}, err
	}
	defer response.Body.Close()

	var turn AgentTurn
	switch provider.Type {
	case ProviderTypeOpenAI:
		err = readOpenAIAgentStream(response.Body, &turn, handle)
	case ProviderTypeAnthropic:
		err = readAnthropicAgentStream(response.Body, &turn, handle)
	}
	if err != nil {
		return AgentTurn{}, err
	}
	for index := range turn.ToolCalls {
		if strings.TrimSpace(turn.ToolCalls[index].ID) == "" {

			turn.ToolCalls[index].ID = fmt.Sprintf("agentcall_%d", index+1)
		}
	}
	return turn, nil
}

var ErrToolsUnsupported = errors.New("provider does not support native tools")

var nativeToolsRejected sync.Map

func providerToolsKey(provider Provider) string {
	return string(provider.Type) + "|" + strings.TrimSpace(provider.BaseURL) + "|" + strings.TrimSpace(provider.Model)
}

func markNativeToolsRejected(provider Provider, err error) {
	var status *StatusError
	if err == nil || !errors.As(err, &status) || status.Status != http.StatusBadRequest {
		return
	}
	detail := strings.ToLower(status.Detail)
	if strings.Contains(detail, "tool_choice") || strings.Contains(detail, "tool choice") || strings.Contains(detail, "tools") {
		nativeToolsRejected.Store(providerToolsKey(provider), struct{}{})
	}
}

func openAIAgentMessages(messages []AgentMessage) []map[string]any {
	converted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		switch {
		case len(message.ToolCalls) > 0:
			calls := make([]map[string]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				calls = append(calls, map[string]any{
					"id":   call.ID,
					"type": "function",
					"function": map[string]any{
						"name":      call.Name,
						"arguments": call.Arguments,
					},
				})
			}
			converted = append(converted, map[string]any{
				"role":       "assistant",
				"content":    message.Content,
				"tool_calls": calls,
			})
		case message.Role == "tool":
			converted = append(converted, map[string]any{
				"role":         "tool",
				"tool_call_id": message.ToolCallID,
				"content":      message.Content,
			})
		default:
			converted = append(converted, map[string]any{
				"role":    message.Role,
				"content": message.Content,
			})
		}
	}
	return converted
}

func openAITools(tools []ToolSpec) []map[string]any {
	converted := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		converted = append(converted, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return converted
}

func anthropicAgentMessages(messages []AgentMessage) (string, []map[string]any) {
	system := ""
	converted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message.Role == "system" && len(message.ToolCalls) == 0 {
			system = strings.TrimSpace(system + "\n" + message.Content)
			continue
		}
		switch {
		case len(message.ToolCalls) > 0:
			blocks := make([]map[string]any, 0, len(message.ToolCalls)+1)
			if strings.TrimSpace(message.Content) != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": message.Content})
			}
			for _, call := range message.ToolCalls {
				input := map[string]any{}
				if strings.TrimSpace(call.Arguments) != "" {
					_ = json.Unmarshal([]byte(call.Arguments), &input)
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    call.ID,
					"name":  call.Name,
					"input": input,
				})
			}
			converted = append(converted, map[string]any{"role": "assistant", "content": blocks})
		case message.Role == "tool":
			converted = append(converted, map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": message.ToolCallID,
					"content":     []map[string]any{{"type": "text", "text": message.Content}},
				}},
			})
		default:
			converted = append(converted, map[string]any{"role": message.Role, "content": message.Content})
		}
	}
	return system, converted
}

func anthropicTools(tools []ToolSpec) []map[string]any {
	converted := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		converted = append(converted, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.Parameters,
		})
	}
	return converted
}

type toolCallAccumulator struct {
	order []int
	calls map[int]*ToolCall
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{calls: make(map[int]*ToolCall)}
}

func (a *toolCallAccumulator) ensure(index int) *ToolCall {
	if call, ok := a.calls[index]; ok {
		return call
	}
	call := &ToolCall{}
	a.calls[index] = call
	a.order = append(a.order, index)
	return call
}

func (a *toolCallAccumulator) result() []ToolCall {
	if len(a.order) == 0 {
		return nil
	}
	calls := make([]ToolCall, 0, len(a.order))
	for _, index := range a.order {
		calls = append(calls, *a.calls[index])
	}
	return calls
}

func readOpenAIAgentStream(body io.Reader, turn *AgentTurn, handle func(AgentDelta) error) error {
	accumulator := newToolCallAccumulator()
	err := readAgentSSE(body, func(data string) error {
		var event struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *openAIUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {

			return nil
		}
		usage := event.Usage.toUsage()
		if len(event.Choices) > 0 {
			delta := event.Choices[0].Delta
			turn.Content += delta.Content
			turn.Reasoning += delta.ReasoningContent
			for _, call := range delta.ToolCalls {
				slot := accumulator.ensure(call.Index)
				if call.ID != "" {
					slot.ID = call.ID
				}
				if call.Function.Name != "" {
					slot.Name += call.Function.Name
				}
				slot.Arguments += call.Function.Arguments
			}
			if handle != nil && (delta.Content != "" || delta.ReasoningContent != "" || usage != nil) {
				if err := handle(AgentDelta{Content: delta.Content, Reasoning: delta.ReasoningContent, Usage: usage}); err != nil {
					return err
				}
			}
		} else if usage != nil && handle != nil {
			if err := handle(AgentDelta{Usage: usage}); err != nil {
				return err
			}
		}
		if usage != nil {
			mergeUsage(&turn.Usage, usage)
		}
		return nil
	})
	if err != nil {
		return err
	}
	turn.ToolCalls = accumulator.result()
	return nil
}

func readAnthropicAgentStream(body io.Reader, turn *AgentTurn, handle func(AgentDelta) error) error {
	accumulator := newToolCallAccumulator()
	err := readAgentSSE(body, func(data string) error {
		var event struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Message *struct {
				Usage *anthropicUsage `json:"usage"`
			} `json:"message"`
			Usage *anthropicUsage `json:"usage"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return nil
		}
		switch event.Type {
		case "message_start":
			if event.Message != nil {
				if usage := event.Message.Usage.toUsage(); usage != nil {
					mergeUsage(&turn.Usage, usage)
				}
			}
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				slot := accumulator.ensure(event.Index)
				slot.ID = event.ContentBlock.ID
				slot.Name = event.ContentBlock.Name
				if len(event.ContentBlock.Input) > 0 && string(event.ContentBlock.Input) != "{}" {
					slot.Arguments = string(event.ContentBlock.Input)
				}
			}
		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				turn.Content += event.Delta.Text
				if handle != nil && event.Delta.Text != "" {
					return handle(AgentDelta{Content: event.Delta.Text})
				}
			case "thinking_delta":
				turn.Reasoning += event.Delta.Thinking
				if handle != nil && event.Delta.Thinking != "" {
					return handle(AgentDelta{Reasoning: event.Delta.Thinking})
				}
			case "input_json_delta":
				slot := accumulator.ensure(event.Index)
				slot.Arguments += event.Delta.PartialJSON
			}
		case "message_delta":
			if usage := event.Usage.toUsage(); usage != nil {
				mergeUsage(&turn.Usage, usage)
				if handle != nil {
					return handle(AgentDelta{Usage: usage})
				}
			}
		case "error":
			if event.Error != nil {
				return &StatusError{Status: http.StatusBadGateway, Detail: strings.TrimSpace(event.Error.Message)}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	turn.ToolCalls = accumulator.result()
	return nil
}

func readAgentSSE(body io.Reader, onEvent func(data string) error) error {
	reader := bufio.NewReaderSize(body, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data == "[DONE]" {
					return nil
				}
				if data != "" {
					if handlerErr := onEvent(data); handlerErr != nil {
						return handlerErr
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

func mergeUsage(target **Usage, source *Usage) {
	if source == nil {
		return
	}
	if *target == nil {
		*target = &Usage{}
	}
	copyValue := func(destination **int64, value *int64) {
		if value != nil {
			copied := *value
			*destination = &copied
		}
	}
	copyValue(&(*target).InputTokens, source.InputTokens)
	copyValue(&(*target).OutputTokens, source.OutputTokens)
	copyValue(&(*target).TotalTokens, source.TotalTokens)
	copyValue(&(*target).CacheReadTokens, source.CacheReadTokens)
	copyValue(&(*target).CacheWriteTokens, source.CacheWriteTokens)
}
