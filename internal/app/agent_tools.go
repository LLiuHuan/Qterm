package app

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	agentHostNotesRune = 96

	agentToolObservationBytes = 6 << 10
)

type agentHostView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Username string   `json:"username,omitempty"`
	Group    string   `json:"group,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

type agentCurrentTarget struct {
	SessionID   string `json:"sessionId,omitempty"`
	SessionKind string `json:"sessionKind,omitempty"`
	HostID      string `json:"hostId,omitempty"`
	Name        string `json:"name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Address     string `json:"address,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
}

type agentListHostsResult struct {
	Hosts []agentHostView `json:"hosts"`
	Total int             `json:"total"`

	Current *agentCurrentTarget `json:"current,omitempty"`

	Truncated bool `json:"truncated"`
}

func (executor sessionAgentExecutor) executeAssetTool(_ context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	if executor.hosts == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	switch request.Kind {
	case ai.StepKindListHosts:
		return executor.listHostsTool(request.Args, request.SessionID)
	case ai.StepKindGetHost:
		return executor.getHostTool(request.Args)
	default:
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
}

func (executor sessionAgentExecutor) currentTarget(sessionID string, hosts []model.Host) *agentCurrentTarget {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || executor.sessions == nil {
		return nil
	}
	session, err := executor.sessions.GetSession(context.Background(), sessionID)
	if err != nil {
		return nil
	}
	return projectCurrentTarget(session, hosts)
}

func projectCurrentTarget(session model.Session, hosts []model.Host) *agentCurrentTarget {
	target := &agentCurrentTarget{
		SessionID:   session.ID,
		SessionKind: string(session.Kind),
		HostID:      session.HostID,
	}
	for _, host := range hosts {
		if host.ID != session.HostID {
			continue
		}
		target.Name = host.Name
		target.Protocol = host.Protocol
		target.Address = host.Address
		target.Port = host.Port
		target.Username = host.Username
		break
	}
	return target
}

func (executor sessionAgentExecutor) listHostsTool(raw json.RawMessage, sessionID string) (ai.StepResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	hosts, err := executor.hosts.ListHosts()
	if err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeStoreFailed, "ai.agent.tool.hosts_failed", err)
	}
	matched := filterAgentHosts(hosts, args.Query)
	views := make([]agentHostView, 0, min(limit, len(matched)))
	for _, host := range matched {
		if len(views) >= limit {
			break
		}
		views = append(views, projectAgentHost(host))
	}

	result := agentListHostsResult{
		Hosts:     views,
		Total:     len(matched),
		Truncated: len(views) < len(matched),
		Current:   executor.currentTarget(sessionID, hosts),
	}
	encoded, err := marshalAgentListResult(result)
	if err != nil {
		return ai.StepResult{}, err
	}
	return ai.StepResult{Observation: encoded}, nil
}

func (executor sessionAgentExecutor) getHostTool(raw json.RawMessage) (ai.StepResult, error) {
	var args struct {
		HostID string `json:"host_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	hostID := strings.TrimSpace(args.HostID)
	if hostID == "" {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	hosts, err := executor.hosts.ListHosts()
	if err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeStoreFailed, "ai.agent.tool.hosts_failed", err)
	}
	for _, host := range hosts {
		if host.ID != hostID {
			continue
		}
		encoded, marshalErr := marshalAgentObservation(projectAgentHost(host))
		if marshalErr != nil {
			return ai.StepResult{}, marshalErr
		}
		return ai.StepResult{Observation: encoded}, nil
	}

	encoded, marshalErr := marshalAgentObservation(struct {
		Error  string `json:"error"`
		HostID string `json:"hostId"`
	}{Error: "host_not_found", HostID: hostID})
	if marshalErr != nil {
		return ai.StepResult{}, marshalErr
	}
	return ai.StepResult{Observation: encoded}, nil
}

func filterAgentHosts(hosts []model.Host, query string) []model.Host {
	keyword := strings.ToLower(strings.TrimSpace(query))
	if keyword == "" {
		return hosts
	}
	matched := make([]model.Host, 0, len(hosts))
	for _, host := range hosts {
		if agentHostMatches(host, keyword) {
			matched = append(matched, host)
		}
	}
	return matched
}

func agentHostMatches(host model.Host, keyword string) bool {
	fields := []string{host.Name, host.Address, host.Username, host.Group}
	for _, tag := range host.Tags {
		fields = append(fields, tag)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

func projectAgentHost(host model.Host) agentHostView {
	return agentHostView{
		ID:       host.ID,
		Name:     host.Name,
		Protocol: host.Protocol,
		Address:  host.Address,
		Port:     host.Port,
		Username: host.Username,
		Group:    host.Group,
		Tags:     append([]string(nil), host.Tags...),
		Notes:    truncateRunes(strings.TrimSpace(host.Notes), agentHostNotesRune),
	}
}

func marshalAgentListResult(result agentListHostsResult) (string, error) {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return "", errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
		}
		if len(encoded) <= agentToolObservationBytes || len(result.Hosts) == 0 {
			return string(encoded), nil
		}
		result.Hosts = result.Hosts[:len(result.Hosts)-1]
		result.Truncated = true
	}
}

func marshalAgentObservation(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
	}
	return string(encoded), nil
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}
