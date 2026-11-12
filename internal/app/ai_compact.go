package app

import (
	"context"

	"changeme/internal/ai"
)

type AIConversationCompaction struct {
	Compacted    bool         `json:"compacted"`
	Reason       string       `json:"reason,omitempty"`
	Summary      string       `json:"summary,omitempty"`
	KeptMessages []ai.Message `json:"keptMessages,omitempty"`
	BeforeBytes  int          `json:"beforeBytes"`
	AfterBytes   int          `json:"afterBytes"`
}

func (s *Service) CompactAIConversation(
	ctx context.Context,
	input AIChatInput,
	keepRecent int,
	priorSummary string,
) (AIConversationCompaction, error) {
	if s == nil || s.ai == nil {
		return AIConversationCompaction{}, publicError(ai.ErrNotConfigured)
	}
	result, err := s.ai.Compact(ctx, ai.CompactRequest{
		Provider: ai.Provider{
			ID:      input.ProviderID,
			Type:    input.Type,
			BaseURL: input.BaseURL,
			Model:   input.Model,
		},
		Messages:     input.Messages,
		KeepRecent:   keepRecent,
		PriorSummary: priorSummary,
	})
	if err != nil {
		return AIConversationCompaction{}, publicError(err)
	}
	return AIConversationCompaction{
		Compacted:    result.Compacted,
		Reason:       result.Reason,
		Summary:      result.Summary,
		KeptMessages: result.Kept,
		BeforeBytes:  result.BeforeBytes,
		AfterBytes:   result.AfterBytes,
	}, nil
}
