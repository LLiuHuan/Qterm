package model

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidAIContextSettings = errors.New("invalid ai context settings")

const (
	MaxAICustomPromptLength = 4096
)

type AIContextSettings struct {
	CustomPrompt string `json:"customPrompt"`

	IncludeHostNote bool `json:"includeHostNote"`

	IncludeSnippetIndex bool `json:"includeSnippetIndex"`
}

func NormalizeAIContextSettings(input AIContextSettings) (AIContextSettings, error) {
	prompt := strings.TrimSpace(input.CustomPrompt)
	if len(prompt) > MaxAICustomPromptLength*4 || !utf8.ValidString(prompt) || strings.IndexByte(prompt, 0) >= 0 {
		return AIContextSettings{}, &ValidationError{Field: "customPrompt", Reason: "长度或字符不合法", Err: ErrInvalidAIContextSettings}
	}

	if utf8.RuneCountInString(prompt) > MaxAICustomPromptLength {
		return AIContextSettings{}, &ValidationError{Field: "customPrompt", Reason: "自定义提示词过长", Err: ErrInvalidAIContextSettings}
	}
	return AIContextSettings{
		CustomPrompt:        prompt,
		IncludeHostNote:     input.IncludeHostNote,
		IncludeSnippetIndex: input.IncludeSnippetIndex,
	}, nil
}
