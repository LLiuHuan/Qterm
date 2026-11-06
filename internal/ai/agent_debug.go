package ai

import (
	"fmt"
	"strconv"
	"strings"
)

type AgentDebugFunc func(runID, event, detail string)

const (
	debugExcerptBytes = 2 << 10
	debugCommandBytes = 1 << 10
)

func debugExcerpt(value string) string {
	redacted := redactAgentContext(value)
	truncated, _ := truncateUTF8(redacted, debugExcerptBytes)
	return strings.ReplaceAll(truncated, "\n", "\\n")
}

func debugCommand(command string) string {
	redacted := redactAgentCommand(command)
	truncated, _ := truncateUTF8(redacted, debugCommandBytes)
	return strings.ReplaceAll(truncated, "\n", "\\n")
}

func debugUsage(usage *Usage) string {
	if usage == nil {
		return "none"
	}
	format := func(value *int64) string {
		if value == nil {
			return "-"
		}
		return strconv.FormatInt(*value, 10)
	}
	return fmt.Sprintf("in=%s out=%s total=%s", format(usage.InputTokens), format(usage.OutputTokens), format(usage.TotalTokens))
}

func (r *Runner) debug(runID, event, detail string) {
	if r == nil || r.debugLog == nil {
		return
	}
	r.debugLog(runID, event, detail)
}

func (r *Runner) debugTurn(runID, mode string, turn AgentTurn) {
	if r == nil || r.debugLog == nil {
		return
	}
	r.debug(runID, "turn.response", fmt.Sprintf(
		"mode=%s toolCalls=%d contentBytes=%d reasoningBytes=%d usage=%s",
		mode, len(turn.ToolCalls), len(turn.Content), len(turn.Reasoning), debugUsage(turn.Usage),
	))
	for index, call := range turn.ToolCalls {
		if index >= 4 {
			break
		}
		r.debug(runID, "turn.tool_call", fmt.Sprintf("index=%d id=%s name=%s arguments=%s", index, call.ID, call.Name, debugExcerpt(call.Arguments)))
	}
	if strings.TrimSpace(turn.Content) != "" {
		r.debug(runID, "turn.content", debugExcerpt(turn.Content))
	}
}

func boolMode(native bool) string {
	if native {
		return "native"
	}
	return "json"
}
