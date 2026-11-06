package ai

import (
	"regexp"
	"strings"
	"time"
)

const (
	AuditDecisionAuto = "auto"

	AuditDecisionNeedsApproval = "needs_approval"

	AuditDecisionApproved = "approved"
	AuditDecisionRejected = "rejected"
)

type AgentAuditEntry struct {
	RunID  string `json:"runId"`
	StepID string `json:"stepId"`
	CallID string `json:"callId,omitempty"`

	SessionID string `json:"sessionId"`

	Command string `json:"command"`

	Risk string `json:"risk"`

	Decision string `json:"decision"`
	State    string `json:"state"`

	ExitCode   *int       `json:"exitCode,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	DurationMS int64      `json:"durationMs,omitempty"`
}

var (
	auditPasswordPattern = regexp.MustCompile(`(?i)((?:--?p(?:ass(?:word)?)?|--token|-t)[= ]+)(\S+)`)
	auditBearerPattern   = regexp.MustCompile(`(?i)(bearer\s+)(\S+)`)
	auditURLPassword     = regexp.MustCompile(`(://[^:/\s@]+:)([^@\s]+)(@)`)
	auditSecretPair      = regexp.MustCompile(`(?i)((?:password|passwd|token|secret|api[_-]?key)\s*[=:]\s*)(\S+)`)

	auditAttachedPassword = regexp.MustCompile(`(\s-p)([^\s/][^\s]{2,})`)
)

func redactAgentCommand(command string) string {
	trimmed := strings.TrimSpace(command)
	trimmed = auditPasswordPattern.ReplaceAllString(trimmed, "${1}***")
	trimmed = auditBearerPattern.ReplaceAllString(trimmed, "${1}***")
	trimmed = auditURLPassword.ReplaceAllString(trimmed, "${1}***${3}")
	trimmed = auditSecretPair.ReplaceAllString(trimmed, "${1}***")
	trimmed = auditAttachedPassword.ReplaceAllString(trimmed, "${1}***")
	if len([]rune(trimmed)) > 200 {
		trimmed = string([]rune(trimmed)[:200]) + "…"
	}
	return trimmed
}

func (m *Manager) auditFor(runID, stepID string) *AgentAuditEntry {
	for index := len(m.audit) - 1; index >= 0; index-- {
		entry := &m.audit[index]
		if entry.RunID == runID && entry.StepID == stepID {
			return entry
		}
	}
	return nil
}

func (m *Manager) appendAuditLocked(entry AgentAuditEntry) {
	m.audit = append(m.audit, entry)
	if overflow := len(m.audit) - m.maxAudit; overflow > 0 {
		trimmed := make([]AgentAuditEntry, len(m.audit)-overflow)
		copy(trimmed, m.audit[overflow:])
		m.audit = trimmed
	}
}

func (m *Manager) Audit(limit int) []AgentAuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.audit) {
		limit = len(m.audit)
	}
	entries := make([]AgentAuditEntry, 0, limit)
	for index := len(m.audit) - 1; index >= 0 && len(entries) < limit; index-- {
		entry := m.audit[index]
		entry.ExitCode = cloneInt(entry.ExitCode)
		entry.FinishedAt = cloneTime(entry.FinishedAt)
		entries = append(entries, entry)
	}
	return entries
}

func (m *Manager) noteAuditDecision(runID, stepID, decision string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry := m.auditFor(runID, stepID); entry != nil {
		entry.Decision = decision
	}
}

func (m *Manager) finishAuditLocked(runID, stepID string, state StepState, exitCode *int, now time.Time) {
	entry := m.auditFor(runID, stepID)
	if entry == nil {
		return
	}
	entry.State = string(state)
	entry.ExitCode = cloneInt(exitCode)
	finished := now
	entry.FinishedAt = &finished
	if !entry.StartedAt.IsZero() {
		entry.DurationMS = now.Sub(entry.StartedAt).Milliseconds()
	}
}

func (m *Manager) RestoreAudit(entries []AgentAuditEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range entries {
		if strings.TrimSpace(entry.RunID) == "" || strings.TrimSpace(entry.StepID) == "" {
			continue
		}
		if m.hasAuditLocked(entry.RunID, entry.StepID) {
			continue
		}
		entry.ExitCode = cloneInt(entry.ExitCode)
		entry.FinishedAt = cloneTime(entry.FinishedAt)
		m.appendAuditLocked(entry)
	}
}

func (m *Manager) hasAuditLocked(runID, stepID string) bool {
	for index := len(m.audit) - 1; index >= 0; index-- {
		if m.audit[index].RunID == runID && m.audit[index].StepID == stepID {
			return true
		}
	}
	return false
}
