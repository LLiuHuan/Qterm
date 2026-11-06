package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"changeme/internal/ai"
)

const (
	agentAuditFileName = "ai-agent-audit.json"
	agentAuditVersion  = 1
)

type agentAuditFile struct {
	Version int                  `json:"version"`
	Entries []ai.AgentAuditEntry `json:"entries"`
}

func (s *Service) setupAgentAudit() {
	if s == nil || s.agents == nil || s.security == nil {
		return
	}
	root := s.security.Root()
	if root == "" {
		return
	}
	s.agentAuditPath = filepath.Join(root, agentAuditFileName)
	s.loadAgentAudit(s.agentAuditPath)
}

func (s *Service) loadAgentAudit(path string) {
	if s == nil || s.agents == nil || path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var payload agentAuditFile
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Warn("ai agent audit file is corrupted", "error", err.Error())
		return
	}
	s.agents.RestoreAudit(payload.Entries)
}

func (s *Service) persistAgentAudit() {
	if s == nil || s.agents == nil || s.agentAuditPath == "" {
		return
	}
	s.agentAuditMu.Lock()
	defer s.agentAuditMu.Unlock()
	payload := agentAuditFile{Version: agentAuditVersion, Entries: s.agents.Audit(0)}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	temp := s.agentAuditPath + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		slog.Warn("write ai agent audit failed", "error", err.Error())
		return
	}
	if err := os.Rename(temp, s.agentAuditPath); err != nil {
		slog.Warn("replace ai agent audit failed", "error", err.Error())
	}
}
