package app

import (
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const agentDraftEvent = "qterm:agent-draft"

type agentDraftPayload struct {
	RunID     string `json:"runId"`
	Draft     string `json:"draft"`
	Reasoning string `json:"reasoning"`
}

type agentDraftThrottle struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (s *Service) emitAgentDraft(runID string) {
	app := application.Get()
	if app == nil || s.agents == nil {
		return
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return
	}
	now := time.Now()
	s.agentDraft.mu.Lock()
	previous, seen := s.agentDraft.last[runID]
	if seen && now.Sub(previous) < agentDraftThrottleInterval {
		s.agentDraft.mu.Unlock()
		return
	}
	if s.agentDraft.last == nil {
		s.agentDraft.last = map[string]time.Time{}
	}
	s.agentDraft.last[runID] = now
	s.agentDraft.mu.Unlock()

	draft, reasoning := s.agents.DraftSnapshot(runID)
	app.Event.Emit(agentDraftEvent, agentDraftPayload{RunID: runID, Draft: draft, Reasoning: reasoning})
}

const agentDraftThrottleInterval = 60 * time.Millisecond
