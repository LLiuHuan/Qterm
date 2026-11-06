package ai

import (
	"regexp"
	"strings"
)

const (
	defaultAgentPromptBytes      = 96 << 10
	minAgentPromptBytes          = 4 << 10
	compressedObservationBytes   = 1 << 10
	maxAgentContextInjectedBytes = 8 << 10
	truncatedObservationMarker   = "\n[observation truncated to fit the context budget]"
)

var (
	privateKeyPattern = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)

	credentialTokenPattern = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{16,}|ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,})\b`)
)

func redactAgentContext(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	redacted := privateKeyPattern.ReplaceAllString(text, "[redacted private key]")
	redacted = auditPasswordPattern.ReplaceAllString(redacted, "${1}***")
	redacted = auditBearerPattern.ReplaceAllString(redacted, "${1}***")
	redacted = auditURLPassword.ReplaceAllString(redacted, "${1}***${3}")
	redacted = auditSecretPair.ReplaceAllString(redacted, "${1}***")
	redacted = auditAttachedPassword.ReplaceAllString(redacted, "${1}***")
	redacted = credentialTokenPattern.ReplaceAllString(redacted, "***")
	redacted, _ = truncateUTF8(redacted, maxAgentContextInjectedBytes)
	return redacted
}

func agentMessageBytes(message AgentMessage) int {
	size := len(message.Content) + len(message.ToolCallID) + len(message.Name) + 32
	for _, call := range message.ToolCalls {
		size += len(call.ID) + len(call.Name) + len(call.Arguments) + 32
	}
	return size
}

func agentMessagesBytes(messages []AgentMessage) int {
	total := 0
	for _, message := range messages {
		total += agentMessageBytes(message)
	}
	return total
}

func compressAgentMessages(messages []AgentMessage, limit int) []AgentMessage {
	if limit <= 0 || len(messages) == 0 {
		return messages
	}
	compressed := make([]AgentMessage, len(messages))
	copy(compressed, messages)
	total := agentMessagesBytes(compressed)
	if total <= limit {
		return compressed
	}

	for index := 0; index < len(compressed)-1 && total > limit; index++ {
		if compressed[index].Role != "tool" || len(compressed[index].Content) <= compressedObservationBytes {
			continue
		}
		shortened, _ := truncateUTF8(compressed[index].Content, compressedObservationBytes)
		total -= len(compressed[index].Content) - len(shortened)
		compressed[index].Content = shortened + truncatedObservationMarker
		total += len(truncatedObservationMarker)
	}
	if total <= limit {
		return compressed
	}

	start := len(compressed) - 1
	size := agentMessageBytes(compressed[0]) + agentMessageBytes(compressed[start])
	for index := len(compressed) - 2; index >= 1; index-- {
		next := size + agentMessageBytes(compressed[index])
		if next > limit {
			break
		}
		size = next
		start = index
	}
	if start > 1 && compressed[start].Role == "tool" {

		start--
	}
	kept := make([]AgentMessage, 0, len(compressed)-start+1)
	kept = append(kept, compressed[0])
	kept = append(kept, compressed[start:]...)
	return kept
}
