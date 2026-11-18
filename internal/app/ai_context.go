package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	aiContextPreferenceKey = "qterm.aiContext.v1"

	maxSnippetIndexEntries = 80
	maxSnippetIndexBytes   = 6 << 10

	maxSnippetLineBytes = 200
)

func (s *Service) AIContextSettings(ctx context.Context) (model.AIContextSettings, error) {
	settings, err := s.loadAIContextSettings(ctx)
	if err != nil {
		return model.AIContextSettings{}, publicError(err)
	}
	return settings, nil
}

func (s *Service) SetAIContextSettings(ctx context.Context, input model.AIContextSettings) (model.AIContextSettings, error) {
	settings, err := model.NormalizeAIContextSettings(input)
	if err != nil {
		return model.AIContextSettings{}, publicError(err)
	}
	if s.preferences == nil {
		return model.AIContextSettings{}, publicError(errno.New(errno.CodeStoreFailed, "ai.context.unavailable", ""))
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return model.AIContextSettings{}, publicError(errno.New(errno.CodeStoreFailed, "ai.context.save_failed", ""))
	}
	if err := s.preferences.Set(ctx, aiContextPreferenceKey, string(payload)); err != nil {
		return model.AIContextSettings{}, publicError(errno.New(errno.CodeStoreFailed, "ai.context.save_failed", ""))
	}
	return settings, nil
}

func (s *Service) loadAIContextSettings(ctx context.Context) (model.AIContextSettings, error) {
	if s == nil || s.preferences == nil {
		return model.AIContextSettings{}, nil
	}
	raw, ok, err := s.preferences.Get(ctx, aiContextPreferenceKey)
	if err != nil {
		return model.AIContextSettings{}, errno.New(errno.CodeStoreFailed, "ai.context.load_failed", "")
	}
	if !ok || strings.TrimSpace(raw) == "" {
		return model.AIContextSettings{}, nil
	}
	var settings model.AIContextSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		slog.Warn("ai context settings are corrupt; falling back to defaults", "error", err)
		return model.AIContextSettings{}, nil
	}
	normalized, err := model.NormalizeAIContextSettings(settings)
	if err != nil {
		slog.Warn("ai context settings are invalid; falling back to defaults", "error", err)
		return model.AIContextSettings{}, nil
	}
	return normalized, nil
}

func (s *Service) aiRunContext(ctx context.Context, settings model.AIContextSettings, sessionID string) (hostNote string, snippetIndex []string) {
	if !settings.IncludeHostNote && !settings.IncludeSnippetIndex {
		return "", nil
	}

	var hostID string
	if s.sessions != nil && strings.TrimSpace(sessionID) != "" {
		if session, err := s.sessions.GetSession(ctx, sessionID); err == nil {
			hostID = session.HostID
		}
	}
	if settings.IncludeHostNote && hostID != "" && s.hosts != nil {
		if records, err := s.hosts.ListHosts(); err == nil {
			for _, record := range records {
				if record.ID == hostID {
					hostNote = strings.TrimSpace(record.Notes)
					break
				}
			}
		}
	}
	if settings.IncludeSnippetIndex && s.hosts != nil {
		if snippets, err := s.hosts.ListSnippets(); err == nil {
			snippetIndex = snippetIndexLines(snippets, hostID)
		}
	}
	return hostNote, snippetIndex
}

func snippetIndexLines(snippets []model.Snippet, hostID string) []string {
	lines := make([]string, 0, len(snippets))
	total := 0
	for _, snippet := range snippets {
		if len(snippet.HostIDs) > 0 && (hostID == "" || !snippetHostApplies(snippet.HostIDs, hostID)) {
			continue
		}
		line := strings.TrimSpace(snippet.Name)
		if command := strings.TrimSpace(snippet.Content); command != "" {
			line += ": " + truncateSnippetLine(command)
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if total+len(line) > maxSnippetIndexBytes {
			break
		}
		total += len(line)
		lines = append(lines, line)
		if len(lines) >= maxSnippetIndexEntries {
			break
		}
	}
	return lines
}

func snippetHostApplies(hostIDs []string, hostID string) bool {
	for _, candidate := range hostIDs {
		if candidate == hostID {
			return true
		}
	}
	return false
}

func truncateSnippetLine(command string) string {
	if index := strings.IndexByte(command, '\n'); index >= 0 {
		command = command[:index] + " …"
	}
	if len(command) > maxSnippetLineBytes {
		command = command[:maxSnippetLineBytes] + "…"
	}
	return command
}
