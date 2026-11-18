package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
)

const aiCommandRulesPreferenceKey = "qterm.aiCommandRules.v1"

const maxAICommandRulesFileSize = 1 << 20

type aiCommandRulesState struct {
	mu       sync.RWMutex
	loaded   bool
	rules    []model.AICommandRule
	compiled []ai.CompiledCommandRule
}

func (s *Service) AICommandRules() ([]model.AICommandRule, error) {
	if s == nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	if err := s.ensureAICommandRules(); err != nil {
		return nil, err
	}
	s.commandRules.mu.RLock()
	defer s.commandRules.mu.RUnlock()
	return cloneAICommandRules(s.commandRules.rules), nil
}

func (s *Service) CreateAICommandRule(input model.AICommandRuleInput) ([]model.AICommandRule, error) {
	if s == nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	rule, err := model.NormalizeAICommandRule(input)
	if err != nil {
		return nil, publicError(err)
	}
	rule.ID = newAICommandRuleID()
	if err := s.ensureAICommandRules(); err != nil {
		return nil, err
	}
	s.commandRules.mu.Lock()

	if len(s.commandRules.rules) >= model.MaxAICommandRules {
		s.commandRules.mu.Unlock()
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.too_many", "")
	}
	rules := append(cloneAICommandRules(s.commandRules.rules), rule)
	if err := s.saveAICommandRulesLocked(rules); err != nil {
		s.commandRules.mu.Unlock()
		return nil, err
	}
	s.commandRules.mu.Unlock()
	return s.AICommandRules()
}

func (s *Service) UpdateAICommandRule(ruleID string, input model.AICommandRuleInput) ([]model.AICommandRule, error) {
	if s == nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	rule, err := model.NormalizeAICommandRule(input)
	if err != nil {
		return nil, publicError(err)
	}
	if err := s.ensureAICommandRules(); err != nil {
		return nil, err
	}
	s.commandRules.mu.Lock()
	rules := cloneAICommandRules(s.commandRules.rules)
	found := false
	for index := range rules {
		if rules[index].ID == ruleID {
			rules[index].Pattern = rule.Pattern
			rules[index].Action = rule.Action
			rules[index].Note = rule.Note
			found = true
			break
		}
	}
	if !found {
		s.commandRules.mu.Unlock()
		return nil, errno.New(errno.CodeNotFound, "ai.rules.not_found", "")
	}
	if err := s.saveAICommandRulesLocked(rules); err != nil {
		s.commandRules.mu.Unlock()
		return nil, err
	}
	s.commandRules.mu.Unlock()
	return s.AICommandRules()
}

func (s *Service) DeleteAICommandRule(ruleID string) ([]model.AICommandRule, error) {
	if s == nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	if err := s.ensureAICommandRules(); err != nil {
		return nil, err
	}
	s.commandRules.mu.Lock()
	rules := cloneAICommandRules(s.commandRules.rules)
	kept := rules[:0]
	for _, rule := range rules {
		if rule.ID != ruleID {
			kept = append(kept, rule)
		}
	}
	if len(kept) == len(rules) {
		s.commandRules.mu.Unlock()
		return nil, errno.New(errno.CodeNotFound, "ai.rules.not_found", "")
	}
	if err := s.saveAICommandRulesLocked(kept); err != nil {
		s.commandRules.mu.Unlock()
		return nil, err
	}
	s.commandRules.mu.Unlock()
	return s.AICommandRules()
}

func (s *Service) ImportAICommandRulesFromFile() ([]model.AICommandRule, error) {
	if s == nil || s.fileDialogs == nil {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.dialog_unavailable", "")
	}
	path, err := s.fileDialogs.ChooseAICommandRulesImportFile()
	if err != nil {
		return nil, publicError(err)
	}
	if strings.TrimSpace(path) == "" {

		return s.AICommandRules()
	}
	imported, err := readAICommandRulesFile(path)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAICommandRules(); err != nil {
		return nil, err
	}
	s.commandRules.mu.Lock()
	rules := cloneAICommandRules(s.commandRules.rules)
	seen := make(map[string]struct{}, len(rules)+len(imported))
	for _, rule := range rules {
		seen[rule.Action+"\x00"+rule.Pattern] = struct{}{}
	}
	for _, rule := range imported {
		key := rule.Action + "\x00" + rule.Pattern
		if _, exists := seen[key]; exists {
			continue
		}
		if len(rules) >= model.MaxAICommandRules {
			break
		}
		seen[key] = struct{}{}
		rule.ID = newAICommandRuleID()
		rules = append(rules, rule)
	}
	if err := s.saveAICommandRulesLocked(rules); err != nil {
		s.commandRules.mu.Unlock()
		return nil, err
	}
	s.commandRules.mu.Unlock()
	return s.AICommandRules()
}

func (s *Service) ExportAICommandRulesToFile() error {
	if s == nil || s.fileDialogs == nil {
		return errno.New(errno.CodeInvalidArg, "ai.rules.dialog_unavailable", "")
	}
	rules, err := s.AICommandRules()
	if err != nil {
		return err
	}
	path, err := s.fileDialogs.ChooseAICommandRulesExportFile()
	if err != nil {
		return publicError(err)
	}
	if strings.TrimSpace(path) == "" {
		return nil
	}
	document := model.AICommandRuleImport{Version: model.AICommandRuleImportVersion, Rules: rules}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "ai.rules.save_failed", "")
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return errno.New(errno.CodeStoreFailed, "ai.rules.save_failed", "")
	}
	return nil
}

func (s *Service) compiledAICommandRules() []ai.CompiledCommandRule {
	if s == nil {
		return nil
	}
	if err := s.ensureAICommandRules(); err != nil {
		slog.Warn("ai command rules unavailable", "error", err)
		return nil
	}
	s.commandRules.mu.RLock()
	defer s.commandRules.mu.RUnlock()
	return append([]ai.CompiledCommandRule(nil), s.commandRules.compiled...)
}

func (s *Service) ensureAICommandRules() error {
	s.commandRules.mu.RLock()
	if s.commandRules.loaded {
		s.commandRules.mu.RUnlock()
		return nil
	}
	s.commandRules.mu.RUnlock()

	s.commandRules.mu.Lock()
	defer s.commandRules.mu.Unlock()
	if s.commandRules.loaded {
		return nil
	}
	rules, err := s.loadAICommandRulesLocked()
	if err != nil {
		return err
	}
	s.commandRules.rules = rules
	s.commandRules.compiled = compileAICommandRules(rules)
	s.commandRules.loaded = true
	return nil
}

func (s *Service) loadAICommandRulesLocked() ([]model.AICommandRule, error) {
	if s.preferences == nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	raw, found, err := s.preferences.Get(context.Background(), aiCommandRulesPreferenceKey)
	if err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.load_failed", "")
	}
	if !found || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var document model.AICommandRuleImport
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "ai.rules.load_failed", "")
	}
	rules := make([]model.AICommandRule, 0, len(document.Rules))
	for _, rule := range document.Rules {
		normalized, normalizeErr := model.NormalizeAICommandRule(model.AICommandRuleInput{
			Pattern: rule.Pattern,
			Action:  rule.Action,
			Note:    rule.Note,
		})
		if normalizeErr != nil {
			return nil, errno.New(errno.CodeStoreFailed, "ai.rules.load_failed", "")
		}
		normalized.ID = strings.TrimSpace(rule.ID)
		if normalized.ID == "" {
			normalized.ID = newAICommandRuleID()
		}
		rules = append(rules, normalized)
	}
	return rules, nil
}

func (s *Service) saveAICommandRulesLocked(rules []model.AICommandRule) error {
	compiled := compileAICommandRules(rules)
	if s.preferences == nil {
		return errno.New(errno.CodeStoreFailed, "ai.rules.unavailable", "")
	}
	document := model.AICommandRuleImport{Version: model.AICommandRuleImportVersion, Rules: rules}
	data, err := json.Marshal(document)
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "ai.rules.save_failed", "")
	}
	if err := s.preferences.Set(context.Background(), aiCommandRulesPreferenceKey, string(data)); err != nil {
		return errno.New(errno.CodeStoreFailed, "ai.rules.save_failed", "")
	}
	s.commandRules.rules = rules
	s.commandRules.compiled = compiled
	s.commandRules.loaded = true
	return nil
}

func compileAICommandRules(rules []model.AICommandRule) []ai.CompiledCommandRule {
	if len(rules) == 0 {
		return nil
	}
	converted := make([]ai.CommandRule, 0, len(rules))
	for _, rule := range rules {
		converted = append(converted, ai.CommandRule{
			ID:      rule.ID,
			Pattern: rule.Pattern,
			Action:  ai.CommandRuleAction(rule.Action),
			Note:    rule.Note,
		})
	}
	compiled, err := ai.CompileCommandRules(converted)
	if err != nil {
		slog.Warn("ai command rules failed to compile", "error", err)
		return nil
	}
	return compiled
}

func readAICommandRulesFile(path string) ([]model.AICommandRule, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
	}
	if info.Size() > maxAICommandRulesFileSize {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
	}
	var entries []model.AICommandRuleInput
	if strings.HasPrefix(trimmed, "[") {
		var rules []model.AICommandRule
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
		}
		for _, rule := range rules {
			entries = append(entries, model.AICommandRuleInput{Pattern: rule.Pattern, Action: rule.Action, Note: rule.Note})
		}
	} else {
		var document model.AICommandRuleImport
		if err := json.Unmarshal(data, &document); err != nil {
			return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
		}
		for _, rule := range document.Rules {
			entries = append(entries, model.AICommandRuleInput{Pattern: rule.Pattern, Action: rule.Action, Note: rule.Note})
		}
	}
	normalized, err := model.NormalizeAICommandRules(entries)
	if err != nil {
		return nil, errno.New(errno.CodeInvalidArg, "ai.rules.file_invalid", "")
	}
	return normalized, nil
}

func newAICommandRuleID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("rule-%d", os.Getpid())
	}
	return "rule-" + hex.EncodeToString(buf)
}

func cloneAICommandRules(rules []model.AICommandRule) []model.AICommandRule {
	return append([]model.AICommandRule(nil), rules...)
}
