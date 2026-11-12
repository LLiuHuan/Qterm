package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/webfetch"
)

const (
	agentWebTextBytes = 6 << 10

	agentWebTimeout = 20 * time.Second
)

func (executor sessionAgentExecutor) executeWebTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	if request.Kind != ai.StepKindWebFetch {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(request.Args, &args); err != nil {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}

	settings := webfetch.DefaultSettings()
	if executor.preferences != nil {
		if raw, found, err := executor.preferences.Get(ctx, webfetch.PreferenceKey); err == nil && found {
			settings = webfetch.DecodeSettings(raw)
		}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, agentWebTimeout)
	defer cancel()
	result, err := webfetch.Fetch(fetchCtx, settings, args.URL)
	if err != nil {
		return webFailureStep(err), nil
	}

	text, truncated := truncateTextToBytes(result.Text, agentWebTextBytes)
	encoded, marshalErr := json.Marshal(struct {
		URL         string `json:"url"`
		Status      int    `json:"status"`
		ContentType string `json:"contentType,omitempty"`
		Title       string `json:"title,omitempty"`
		Text        string `json:"text"`
		Bytes       int64  `json:"bytes"`
		Truncated   bool   `json:"truncated"`
	}{
		URL:         result.URL,
		Status:      result.Status,
		ContentType: result.ContentType,
		Title:       result.Title,
		Text:        text,
		Bytes:       result.Bytes,
		Truncated:   truncated || result.Truncated,
	})
	if marshalErr != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", marshalErr)
	}
	return ai.StepResult{Observation: string(encoded)}, nil
}

func webFailureStep(err error) ai.StepResult {
	code := errno.CodeUnknown
	messageKey := "ai.agent.web.failed"
	hint := "the page could not be fetched; do not retry the same URL unchanged"

	switch {
	case errors.Is(err, webfetch.ErrBlockedAddress):
		code = errno.CodePermissionDenied
		messageKey = "ai.agent.web.blocked_address"
		hint = "the address is loopback or private; the user can allow intranet access in Settings → AI"
	case errors.Is(err, webfetch.ErrInvalidURL):
		code = errno.CodeInvalidArg
		messageKey = "ai.agent.web.invalid_url"
		hint = "the URL is invalid; it must be an absolute http/https URL"
	case errors.Is(err, webfetch.ErrUnsupportedContent):
		code = errno.CodeInvalidArg
		messageKey = "ai.agent.web.unsupported_content"
		hint = "the response is not a text page (for example a PDF or an image); ask the user for another source"
	case errors.Is(err, webfetch.ErrTooManyRedirects):
		code = errno.CodeInvalidArg
		messageKey = "ai.agent.web.too_many_redirects"
		hint = "too many redirects; use the final URL directly"
	case errors.Is(err, context.DeadlineExceeded):
		code = errno.CodeConnFailed
		messageKey = "ai.agent.web.timeout"
		hint = "the request timed out; the site may be slow or unreachable"
	default:
		code = errno.CodeConnFailed
	}

	encoded, marshalErr := json.Marshal(struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		MessageKey string `json:"messageKey,omitempty"`
		Hint       string `json:"hint,omitempty"`
	}{
		Error:      "web_failed",
		Code:       string(code),
		MessageKey: messageKey,
		Hint:       hint,
	})
	if marshalErr != nil {
		return ai.StepResult{Observation: `{"error":"web_failed"}`, Failure: "code=UNKNOWN"}
	}
	return ai.StepResult{
		Observation: string(encoded),
		Failure:     "code=" + string(code) + " key=" + messageKey,
	}
}
