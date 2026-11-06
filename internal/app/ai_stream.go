package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"changeme/internal/ai"
)

const aiStreamTTL = 10 * time.Minute

type aiStreamState struct {
	mu     sync.Mutex
	buffer []byte

	reasoning []byte

	usage *AIUsage
	done  bool

	errorKey    string
	errorDetail string
	cancel      context.CancelFunc
	createdAt   time.Time
}

type aiStreamStore struct {
	mu      sync.Mutex
	streams map[string]*aiStreamState
	seq     int64
}

func newAIStreamStore() *aiStreamStore {
	return &aiStreamStore{streams: make(map[string]*aiStreamState)}
}

func (s *aiStreamStore) create(ctx context.Context) (string, *aiStreamState) {
	s.mu.Lock()
	s.seq++
	id := formatStreamID(s.seq)
	streamCtx, cancel := context.WithCancel(ctx)
	state := &aiStreamState{cancel: cancel, createdAt: time.Now()}
	s.streams[id] = state
	s.mu.Unlock()
	s.evictExpired()

	go func() {
		<-streamCtx.Done()
	}()
	return id, state
}

func (s *aiStreamStore) get(id string) (*aiStreamState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.streams[id]
	return state, ok
}

func (s *aiStreamStore) release(id string) {
	s.mu.Lock()
	state := s.streams[id]
	delete(s.streams, id)
	s.mu.Unlock()
	if state != nil {
		state.cancel()
	}
}

func (s *aiStreamStore) evictExpired() {
	deadline := time.Now().Add(-aiStreamTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, state := range s.streams {
		if state.createdAt.Before(deadline) {
			state.cancel()
			delete(s.streams, id)
		}
	}
}

func formatStreamID(seq int64) string {
	return "aistream-" + itoa(seq)
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}

type AIChatStreamResult struct {
	StreamID string `json:"streamId"`
	ErrorKey string `json:"errorKey,omitempty"`

	Context ai.ContextBreakdown `json:"context"`
}

type AIUsage struct {
	InputTokens      *int64 `json:"inputTokens,omitempty"`
	OutputTokens     *int64 `json:"outputTokens,omitempty"`
	TotalTokens      *int64 `json:"totalTokens,omitempty"`
	CacheReadTokens  *int64 `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens *int64 `json:"cacheWriteTokens,omitempty"`
}

func mergeAIUsage(target **AIUsage, source *ai.Usage) {
	if source == nil {
		return
	}
	if *target == nil {
		*target = &AIUsage{}
	}
	copyInt64 := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		copied := *value
		return &copied
	}
	if source.InputTokens != nil {
		(*target).InputTokens = copyInt64(source.InputTokens)
	}
	if source.OutputTokens != nil {
		(*target).OutputTokens = copyInt64(source.OutputTokens)
	}
	if source.TotalTokens != nil {
		(*target).TotalTokens = copyInt64(source.TotalTokens)
	}
	if source.CacheReadTokens != nil {
		(*target).CacheReadTokens = copyInt64(source.CacheReadTokens)
	}
	if source.CacheWriteTokens != nil {
		(*target).CacheWriteTokens = copyInt64(source.CacheWriteTokens)
	}
}

func cloneAIUsage(source *AIUsage) *AIUsage {
	if source == nil {
		return nil
	}
	copyInt64 := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		copied := *value
		return &copied
	}
	return &AIUsage{
		InputTokens:      copyInt64(source.InputTokens),
		OutputTokens:     copyInt64(source.OutputTokens),
		TotalTokens:      copyInt64(source.TotalTokens),
		CacheReadTokens:  copyInt64(source.CacheReadTokens),
		CacheWriteTokens: copyInt64(source.CacheWriteTokens),
	}
}

func (s *Service) AIChatStream(ctx context.Context, input AIChatInput) (AIChatStreamResult, error) {

	messages, breakdown := aiChatRequestMessages(input)

	streamID, state := s.aiStreams.create(context.Background())

	streamCtx, cancel := context.WithCancel(context.Background())
	state.cancel = cancel

	go func() {
		defer func() {
			state.mu.Lock()
			state.done = true
			state.mu.Unlock()
		}()
		slog.Info("ai stream request",
			"event", "ai.stream_request",
			"provider", input.ProviderID,
			"type", input.Type,
			"model", input.Model,
			"promptBytes", breakdown.Total(),
		)
		err := s.ai.StreamChat(streamCtx, ai.ChatRequest{
			Provider: ai.Provider{ID: input.ProviderID, Type: input.Type, BaseURL: input.BaseURL, Model: input.Model},
			Messages: messages,
			Context:  breakdown,
		}, func(delta string, reasoning string, usage *ai.Usage) error {
			state.mu.Lock()
			state.buffer = append(state.buffer, delta...)
			state.reasoning = append(state.reasoning, reasoning...)
			mergeAIUsage(&state.usage, usage)
			state.mu.Unlock()
			return streamCtx.Err()
		})
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}
		key, detail := aiFailure(err)
		state.mu.Lock()
		state.errorKey = key
		state.errorDetail = detail
		state.mu.Unlock()
	}()

	return AIChatStreamResult{StreamID: streamID, Context: breakdown}, nil
}

type AIChatStreamPollResult struct {
	Content string `json:"content"`

	Reasoning   string   `json:"reasoning"`
	Usage       *AIUsage `json:"usage,omitempty"`
	Done        bool     `json:"done"`
	ErrorKey    string   `json:"errorKey,omitempty"`
	ErrorDetail string   `json:"errorDetail,omitempty"`
}

func (s *Service) AIChatStreamPoll(streamID string) (AIChatStreamPollResult, error) {
	state, ok := s.aiStreams.get(streamID)
	if !ok {
		return AIChatStreamPollResult{Done: true}, nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return AIChatStreamPollResult{
		Content:     string(state.buffer),
		Reasoning:   string(state.reasoning),
		Usage:       cloneAIUsage(state.usage),
		Done:        state.done,
		ErrorKey:    state.errorKey,
		ErrorDetail: state.errorDetail,
	}, nil
}

func (s *Service) AIChatStreamCancel(streamID string) {
	if state, ok := s.aiStreams.get(streamID); ok {
		state.cancel()
	}
}

func (s *Service) AIChatStreamRelease(streamID string) {
	s.aiStreams.release(streamID)
}
