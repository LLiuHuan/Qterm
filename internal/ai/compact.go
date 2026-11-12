package ai

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

const (
	defaultCompactThresholdBytes = 48 << 10

	compactSummaryMaxBytes = 8 << 10

	compactTranscriptPerMessageBytes = 2 << 10

	compactTranscriptTotalBytes = 64 << 10

	compactTranscriptOmittedMarker = "[earlier messages omitted: they did not fit the summarizer budget]\n\n"

	defaultCompactKeepRecent = 4
	minCompactKeepRecent     = 2
	maxCompactKeepRecent     = 20
)

type CompactRequest struct {
	Provider Provider
	Messages []Message

	KeepRecent int

	ThresholdBytes int

	PriorSummary string
}

type CompactResult struct {
	Compacted bool      `json:"compacted"`
	Reason    string    `json:"reason,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Kept      []Message `json:"kept,omitempty"`

	BeforeBytes int `json:"beforeBytes"`
	AfterBytes  int `json:"afterBytes"`
}

var ErrEmptyCompaction = errors.New("compaction produced an empty summary")

func NormalizeCompactKeepRecent(value int) int {
	if value <= 0 {
		return defaultCompactKeepRecent
	}
	if value < minCompactKeepRecent {
		return minCompactKeepRecent
	}
	if value > maxCompactKeepRecent {
		return maxCompactKeepRecent
	}
	return value
}

func (s *Service) Compact(ctx context.Context, request CompactRequest) (CompactResult, error) {
	messages := request.Messages
	before := chatMessagesBytes(messages)
	result := CompactResult{BeforeBytes: before}
	if before == 0 {
		result.Reason = "empty"
		return result, nil
	}
	threshold := request.ThresholdBytes
	if threshold <= 0 {
		threshold = defaultCompactThresholdBytes
	}

	if before <= threshold {
		result.Reason = "below_threshold"
		return result, nil
	}

	keep := request.KeepRecent
	if keep < 0 {
		keep = 0
	} else {
		keep = NormalizeCompactKeepRecent(keep)
	}
	if keep >= len(messages) {

		result.Reason = "nothing_to_compact"
		return result, nil
	}
	older := messages[:len(messages)-keep]
	kept := make([]Message, keep)
	copy(kept, messages[len(messages)-keep:])

	transcript := renderCompactTranscript(older)
	if strings.TrimSpace(transcript) == "" {
		result.Reason = "nothing_to_compact"
		return result, nil
	}

	prompt := []Message{
		{Role: "system", Content: compactSystemPrompt},
		{Role: "user", Content: buildCompactUserPrompt(request.PriorSummary, transcript)},
	}
	summary, err := s.Chat(ctx, ChatRequest{Provider: request.Provider, Messages: prompt})
	if err != nil {

		return CompactResult{}, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {

		return CompactResult{}, ErrEmptyCompaction
	}
	summary, _ = truncateUTF8(summary, compactSummaryMaxBytes)

	result.Compacted = true
	result.Summary = summary
	result.Kept = kept
	result.AfterBytes = len(summary) + chatMessageOverhead + chatMessagesBytes(kept)
	return result, nil
}

const compactSystemPrompt = `You compress a long chat between a user and an SSH assistant into a short handover summary.

Write the summary in the SAME language as the conversation. Keep only what later turns need:
- the user's goals and constraints;
- hosts, paths, commands and configuration values that were established;
- decisions made and the reason they were made;
- what was tried and failed (so it is not retried blindly);
- what is still open.

Do not invent facts, do not add advice, do not use headings or bullet nesting. Write compact prose
or a short flat list, at most 400 words. If something is unknown, omit it rather than guessing.

IMPORTANT when a PRIOR SUMMARY is provided: you are UPDATING it, not replacing it. Carry every fact
from the prior summary forward into the new one unless the new messages explicitly supersede it.
The prior summary already lost detail once; dropping more of it on each pass would eventually leave
nothing useful. Never mention that a summary existed or that this is a merge — output only the new
summary.`

func buildCompactUserPrompt(priorSummary, transcript string) string {
	priorSummary = strings.TrimSpace(priorSummary)
	if priorSummary == "" {
		return transcript
	}
	return "PRIOR SUMMARY (already condensed, carry its facts forward):\n" +
		priorSummary + "\n\nNEW MESSAGES:\n" + transcript
}

func renderCompactTranscript(messages []Message) string {
	chunks := make([]string, 0, len(messages))
	for _, message := range messages {
		content := message.Content
		if message.Role == "user" && len(message.Images) > 0 {
			content += " [附带 " + strconv.Itoa(len(message.Images)) + " 张图片]"
		}
		content, _ = truncateUTF8(strings.TrimSpace(content), compactTranscriptPerMessageBytes)
		if content == "" {
			continue
		}
		label := "USER"
		if message.Role == "assistant" {
			label = "ASSISTANT"
		} else if message.Role == "system" {
			label = "SYSTEM"
		}
		chunks = append(chunks, label+": "+content+"\n\n")
	}
	total := 0
	start := len(chunks)
	for index := len(chunks) - 1; index >= 0; index-- {
		size := len(chunks[index])
		if total+size > compactTranscriptTotalBytes && start != len(chunks) {
			break
		}
		total += size
		start = index
	}
	var builder strings.Builder
	if start > 0 {
		builder.WriteString(compactTranscriptOmittedMarker)
	}
	for _, chunk := range chunks[start:] {
		builder.WriteString(chunk)
	}
	return builder.String()
}

func chatMessagesBytes(messages []Message) int {
	total := 0
	for _, message := range messages {
		total += len(message.Content) + len(message.Role) + chatMessageOverhead
	}
	return total
}
