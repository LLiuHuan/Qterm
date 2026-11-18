package model

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalidAICommandRule = errors.New("invalid ai command rule")

func ruleValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason, Err: ErrInvalidAICommandRule}
}

const (
	AICommandRuleConfirm = "confirm"

	AICommandRuleDeny = "deny"
)

const (
	MaxAICommandRules = 200

	MaxAICommandPatternLength = 512

	MaxAICommandRuleNoteLength = 500
)

type AICommandRule struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	Action  string `json:"action"`
	Note    string `json:"note"`
}

type AICommandRuleInput struct {
	Pattern string `json:"pattern"`
	Action  string `json:"action"`
	Note    string `json:"note"`
}

type AICommandRuleImport struct {
	Version int             `json:"version"`
	Rules   []AICommandRule `json:"rules"`
}

const AICommandRuleImportVersion = 1

func NormalizeAICommandRuleAction(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case AICommandRuleConfirm:
		return AICommandRuleConfirm, nil
	case AICommandRuleDeny:
		return AICommandRuleDeny, nil
	default:
		return "", ruleValidationError("action", "处理方式只能是 confirm 或 deny")
	}
}

func NormalizeAICommandRule(input AICommandRuleInput) (AICommandRule, error) {
	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		return AICommandRule{}, ruleValidationError("pattern", "规则不能为空")
	}
	if !utf8.ValidString(pattern) {
		return AICommandRule{}, ruleValidationError("pattern", "规则必须是合法的 UTF-8")
	}
	if utf8.RuneCountInString(pattern) > MaxAICommandPatternLength {
		return AICommandRule{}, ruleValidationError("pattern", "规则过长")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return AICommandRule{}, ruleValidationError("pattern", "不是合法的正则表达式")
	}
	action, err := NormalizeAICommandRuleAction(input.Action)
	if err != nil {
		return AICommandRule{}, err
	}
	note := strings.TrimSpace(input.Note)
	if !utf8.ValidString(note) {
		return AICommandRule{}, ruleValidationError("note", "备注必须是合法的 UTF-8")
	}
	if utf8.RuneCountInString(note) > MaxAICommandRuleNoteLength {
		return AICommandRule{}, ruleValidationError("note", "备注过长")
	}
	return AICommandRule{Pattern: pattern, Action: action, Note: note}, nil
}

func NormalizeAICommandRules(inputs []AICommandRuleInput) ([]AICommandRule, error) {
	if len(inputs) > MaxAICommandRules {
		return nil, ruleValidationError("rules", "规则数量超出上限")
	}
	rules := make([]AICommandRule, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		rule, err := NormalizeAICommandRule(input)
		if err != nil {
			return nil, err
		}
		key := rule.Action + "\x00" + rule.Pattern
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		rules = append(rules, rule)
	}
	return rules, nil
}
