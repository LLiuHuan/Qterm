package ai

import (
	"regexp"
	"strings"
)

type CommandRuleAction string

const (
	CommandRuleConfirm CommandRuleAction = "confirm"

	CommandRuleDeny CommandRuleAction = "deny"
)

type CommandRule struct {
	ID      string
	Pattern string
	Action  CommandRuleAction
	Note    string
}

type CompiledCommandRule struct {
	Rule CommandRule
	expr *regexp.Regexp
}

type CommandRuleMatch struct {
	Matched bool
	Action  CommandRuleAction
	RuleID  string
	Pattern string
	Note    string
}

func CompileCommandRules(rules []CommandRule) ([]CompiledCommandRule, error) {
	compiled := make([]CompiledCommandRule, 0, len(rules))
	for _, rule := range rules {
		expr, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, CompiledCommandRule{Rule: rule, expr: expr})
	}
	return compiled, nil
}

func MatchCommandRules(command string, rules []CompiledCommandRule) CommandRuleMatch {
	if len(rules) == 0 {
		return CommandRuleMatch{}
	}
	var confirmMatch *CommandRuleMatch
	for _, rule := range rules {
		if !rule.expr.MatchString(command) {
			continue
		}
		match := CommandRuleMatch{
			Matched: true,
			Action:  rule.Rule.Action,
			RuleID:  rule.Rule.ID,
			Pattern: rule.Rule.Pattern,
			Note:    rule.Rule.Note,
		}
		if rule.Rule.Action == CommandRuleDeny {
			return match
		}
		if confirmMatch == nil {
			confirmMatch = &match
		}
	}
	if confirmMatch != nil {
		return *confirmMatch
	}
	return CommandRuleMatch{}
}

func DescribeCommandRuleDenial(match CommandRuleMatch) string {
	reason := "code=COMMAND_RULE_DENIED key=ai.agent.command_rule_denied"
	if note := strings.TrimSpace(match.Note); note != "" {
		reason += " note=" + note
	}
	if match.RuleID != "" {
		reason += " rule=" + match.RuleID
	}
	return reason
}
