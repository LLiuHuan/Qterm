package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/host"
	"changeme/internal/model"
	"changeme/internal/preferences"
	"changeme/internal/remoteedit"
	"changeme/internal/session"
	"changeme/internal/sftp"
)

func (s *Service) AIAgentStart(ctx context.Context, input ai.StartInput) (ai.AgentSnapshot, error) {
	if s == nil || s.agents == nil || s.ai == nil {
		return ai.AgentSnapshot{}, errno.New(errno.CodeUnknown, "ai.agent.manager_closed", "")
	}

	if s.sessions != nil && s.hosts != nil && input.SessionID != "" && input.TargetLabel == "" {
		if session, getErr := s.sessions.GetSession(ctx, input.SessionID); getErr == nil && session.HostID != "" {
			if records, listErr := s.hosts.ListHosts(); listErr == nil {
				for _, host := range records {
					if host.ID != session.HostID {
						continue
					}
					input.TargetLabel = hostTargetLabel(host)
					break
				}
			}
		}
	}

	if s.sessions != nil && input.SessionID != "" {
		if kind, kindErr := s.sessions.SessionKind(ctx, input.SessionID); kindErr == nil {
			input.SessionKind = string(kind)

			if kind != model.SessionKindSSH && input.ExecutionChannel != ai.ExecutionTerminal {
				input.ExecutionChannel = ai.ExecutionTerminal
				s.agentLog("", "start.channel", "non-ssh session forced to terminal channel")
			}
		} else {
			s.agentLog("", "start.kind", kindErr.Error())
		}
	}
	s.agentLog("", "start.request", fmt.Sprintf(
		"provider=%s type=%s model=%s mode=%s channel=%s kind=%s session=%s promptBytes=%d history=%d",
		input.ProviderID, input.ProviderType, input.Model, input.ApprovalMode,
		input.ExecutionChannel, input.SessionKind, input.SessionID, len(input.Prompt), len(input.History),
	))
	snapshot, err := s.agents.Start(ctx, input)
	if err != nil {
		s.agentLog("", "start.error", err.Error())
		return ai.AgentSnapshot{}, publicError(err)
	}
	options := []ai.RunnerOption{}
	if s.agentDebug != nil {
		options = append(options, ai.WithDebugLog(s.agentDebug.Log))
	}

	options = append(options, ai.WithDraftObserver(s.emitAgentDraft))
	runner := ai.NewRunner(s.agents, s.ai, sessionAgentExecutor{
		sessions:     s.sessions,
		hosts:        s.hosts,
		sftp:         s.sftp,
		remoteEdits:  s.remoteEdits,
		preferences:  s.preferences,
		tempSessions: s.tempSessions,
	}, options...)
	runID := snapshot.Run.ID
	s.agentLog(runID, "start.ok", "run created")
	go func() {
		if _, runErr := runner.Run(context.Background(), runID); runErr != nil {

			slog.Warn("ai agent run ended", "runId", runID, "error", runErr)
		}
	}()
	return snapshot, nil
}

func (s *Service) AIAgentSnapshot(runID string, since int) (ai.AgentSnapshot, error) {
	if s == nil || s.agents == nil {
		return ai.AgentSnapshot{}, errno.New(errno.CodeNotFound, "ai.agent.run_not_found", "")
	}
	return publicValue(s.agents.Snapshot(runID, since))
}

func (s *Service) AIAgentDecide(input ai.DecideInput) (ai.AgentSnapshot, error) {
	if s == nil || s.agents == nil {
		return ai.AgentSnapshot{}, errno.New(errno.CodeNotFound, "ai.agent.run_not_found", "")
	}
	snapshot, err := s.agents.Decide(input)
	if err != nil {
		s.agentLog(input.RunID, "decide.error", err.Error())
		return ai.AgentSnapshot{}, publicError(err)
	}
	s.agentLog(input.RunID, "decide.ok", fmt.Sprintf("approve=%v step=%s", input.Approve, input.StepID))
	return snapshot, nil
}

func (s *Service) AIAgentCancel(runID string) (ai.AgentSnapshot, error) {
	if s == nil || s.agents == nil {
		return ai.AgentSnapshot{}, errno.New(errno.CodeNotFound, "ai.agent.run_not_found", "")
	}
	snapshot, err := s.agents.Cancel(runID)
	s.agentLog(runID, "cancel", fmt.Sprintf("error=%v", err))
	return publicValue(snapshot, err)
}

func (s *Service) AIAgentRelease(runID string) error {
	if s == nil || s.agents == nil {
		return nil
	}
	return publicError(s.agents.Release(runID))
}

func (s *Service) agentLog(runID, event, detail string) {
	if s != nil && s.agentDebug != nil {
		s.agentDebug.Log(runID, event, detail)
	}
}

func (s *Service) AIAgentCancelAll() (int, error) {
	if s == nil || s.agents == nil {
		return 0, nil
	}
	return s.agents.CancelAll(), nil
}

func (s *Service) AIAgentAudit(limit int) ([]ai.AgentAuditEntry, error) {
	if s == nil || s.agents == nil {
		return []ai.AgentAuditEntry{}, nil
	}
	entries := s.agents.Audit(limit)

	s.persistAgentAudit()
	return entries, nil
}

type sessionAgentExecutor struct {
	sessions    *session.Service
	hosts       *host.HostService
	sftp        *sftp.Service
	remoteEdits *remoteedit.Manager

	preferences *preferences.Store

	tempSessions *aiTempSessions
}

func (executor sessionAgentExecutor) ExecuteTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	switch request.Kind {
	case ai.StepKindListHosts, ai.StepKindGetHost:
		return executor.executeAssetTool(ctx, request)
	case ai.StepKindSftpList, ai.StepKindSftpReadFile, ai.StepKindSftpWriteFile,
		ai.StepKindSftpUploadFile, ai.StepKindSftpDownloadFile:
		return executor.executeSftpTool(ctx, request)
	case ai.StepKindLocalListDir, ai.StepKindLocalReadFile:
		return executor.executeLocalTool(ctx, request)
	case ai.StepKindWebFetch:
		return executor.executeWebTool(ctx, request)
	default:

		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool_unrouted", string(request.Kind))
	}
}

func (executor sessionAgentExecutor) Execute(ctx context.Context, request ai.ExecRequest) (ai.StepResult, error) {
	if request.Channel == ai.ExecutionTerminal {
		return terminalAgentExecutor{sessions: executor.sessions}.Execute(ctx, request)
	}
	if executor.sessions == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	result, err := executor.sessions.RunAIBackgroundCommand(ctx, request.SessionID, request.Command)
	if err != nil {
		if errors.Is(err, session.ErrAIBackgroundUnsupported) {

			return ai.StepResult{}, fmt.Errorf("%w: %v", ai.ErrCapabilityUnavailable, err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ai.StepResult{}, err
		}
		return ai.StepResult{}, errno.Wrap(errno.CodeConnFailed, "ai.agent.execution_failed", err)
	}

	exitCode := result.ExitCode
	return ai.StepResult{Observation: result.Output, ExitCode: &exitCode}, nil
}

func hostTargetLabel(host model.Host) string {
	name := strings.TrimSpace(host.Name)
	if name == "" {
		name = strings.TrimSpace(host.Address)
	}
	login := strings.TrimSpace(host.Username)
	address := strings.TrimSpace(host.Address)
	if host.Port > 0 {
		address = fmt.Sprintf("%s:%d", address, host.Port)
	}
	parts := make([]string, 0, 3)
	if login != "" && address != "" {
		parts = append(parts, login+"@"+address)
	} else if address != "" {
		parts = append(parts, address)
	}
	if protocol := strings.TrimSpace(host.Protocol); protocol != "" {
		parts = append(parts, protocol)
	}
	if len(parts) == 0 {
		return name
	}
	return fmt.Sprintf("%s（%s）", name, strings.Join(parts, "，"))
}
