package ai

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"
)

type ToolSideEffect string

const (
	SideEffectReadOnly ToolSideEffect = "readonly"

	SideEffectWrite ToolSideEffect = "write"

	SideEffectDestructive ToolSideEffect = "destructive"
)

type ToolDef struct {
	Name string

	Description string

	Parameters map[string]any

	Kind StepKind

	SideEffect ToolSideEffect

	FlatFields []string

	FallbackExample string

	RequiresDecision bool

	SessionKinds []string

	Parse func(ToolCall) (AgentAction, error)
}

var agentToolDefs = []ToolDef{
	{
		Name: ToolExecuteCommand,
		Description: "Run one shell command on an authorized host. " +
			"In global mode pass host_id and the app connects with the saved host configuration when " +
			"no session is open, so you can inspect a machine without asking the user to connect first. " +
			"Never wrap the command in a code fence.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The exact shell command to run, single line preferred.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "One short sentence for the user explaining why this command is needed.",
				},
				"target_session_id": map[string]any{
					"type":        "string",
					"description": "Optional. Must equal the authorized session id; omit to use the current session.",
				},
				"host_id": map[string]any{
					"type": "string",
					"description": "Required in global mode (no session attached): the host to run on. " +
						"An already-open session is reused; otherwise the app connects automatically with the saved " +
						"configuration (a temporary connection closed when idle). If that host needs interactive " +
						"authentication the call fails and the user must connect manually.",
				},
				"risk": map[string]any{
					"type":        "string",
					"enum":        []string{RiskReadOnly, RiskConfirm, RiskDanger},
					"description": "Your own risk assessment. The backend takes the higher of local and model risk.",
				},
				"risk_reason": map[string]any{
					"type":        "string",
					"description": "Short reason when risk is confirm or danger.",
				},
			},
			"required": []string{"command", "description"},
		},
		Kind:            StepKindCommand,
		SideEffect:      SideEffectWrite,
		FlatFields:      []string{"command", "description", "target_session_id", "host_id", "risk", "risk_reason"},
		FallbackExample: `{"action":"execute_command","arguments":{"command":"df -h","description":"check disk usage","risk":"readonly"}}`,
		Parse:           parseExecuteCommandTool,
	},
	{
		Name: ToolListHosts,
		Description: "List the user's saved hosts (assets). Read-only and needs no approval. " +
			"Use it to find a host by name, address, tag or group before acting on it. " +
			"The result also contains `current`: the session and host this run is attached to right now " +
			"(omitted in global mode). Call it whenever you are unsure which machine you are operating on.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Optional keyword matched against name, address, username, group and tags.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     maxAgentHostLimit,
					"default":     defaultAgentHostLimit,
					"description": "How many hosts to return at most.",
				},
			},
			"required": []string{},
		},
		Kind:            StepKindListHosts,
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"query", "limit"},
		FallbackExample: `{"action":"list_hosts","arguments":{"query":"web","limit":20}}`,
		Parse:           parseListHostsTool,
	},
	{
		Name: ToolGetHost,
		Description: "Get one saved host by id. Read-only and needs no approval. " +
			"Use it after list_hosts when you need that host's full record.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"host_id": map[string]any{
					"type":        "string",
					"description": "Host id exactly as returned by list_hosts.",
				},
			},
			"required": []string{"host_id"},
		},
		Kind:            StepKindGetHost,
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"host_id"},
		FallbackExample: `{"action":"get_host","arguments":{"host_id":"host-123"}}`,
		Parse:           parseGetHostTool,
	},
	{
		Name: ToolSftpList,
		Description: "List one directory on the authorized SSH host. Read-only and needs no approval. " +
			"Use it to find files before reading them. Requires an SSH session; other session kinds do not support it.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute remote directory path, for example /etc or /var/log.",
				},
			},
			"required": []string{"path"},
		},
		Kind:            StepKindSftpList,
		SessionKinds:    []string{"ssh"},
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"path"},
		FallbackExample: `{"action":"sftp_list","arguments":{"path":"/etc"}}`,
		Parse:           parseSftpListTool,
	},
	{
		Name: ToolSftpReadFile,
		Description: "Read a text file on the authorized SSH host. Read-only and needs no approval. " +
			"Use it for configuration or log files; binary files and huge files are reported instead of dumped. " +
			"Requires an SSH session.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute remote file path, for example /etc/nginx/nginx.conf.",
				},
			},
			"required": []string{"path"},
		},
		Kind:            StepKindSftpReadFile,
		SessionKinds:    []string{"ssh"},
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"path"},
		FallbackExample: `{"action":"sftp_read_file","arguments":{"path":"/etc/hosts"}}`,
		Parse:           parseSftpReadFileTool,
	},
	{
		Name: ToolSftpWriteFile,
		Description: "Write a text file on the authorized SSH host. This changes the remote file and always " +
			"asks the user to approve the diff first. Prefer small, focused edits; write at most " +
			"64 KiB of UTF-8 text per call.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute remote file path. A missing file is created.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The complete new file content (UTF-8).",
				},
			},
			"required": []string{"path", "content"},
		},
		Kind:            StepKindSftpWriteFile,
		SessionKinds:    []string{"ssh"},
		SideEffect:      SideEffectWrite,
		FlatFields:      []string{"path", "content"},
		FallbackExample: `{"action":"sftp_write_file","arguments":{"path":"/etc/hosts","content":"127.0.0.1 localhost"}}`,
		Parse:           parseSftpWriteFileTool,
	},
	{
		Name: ToolSftpUploadFile,
		Description: "Upload a LOCAL file to the authorized SSH host. This writes a remote file and always " +
			"asks the user to approve the exact local and remote paths first. Refuses to overwrite an " +
			"existing remote file: pick a different remote path instead. At most 64 MiB per call.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"localPath": map[string]any{
					"type":        "string",
					"description": "Absolute path of the local file on the machine running Qterm.",
				},
				"remotePath": map[string]any{
					"type":        "string",
					"description": "Absolute destination path on the SSH host. Must not exist yet.",
				},
			},
			"required": []string{"localPath", "remotePath"},
		},
		Kind:            StepKindSftpUploadFile,
		SessionKinds:    []string{"ssh"},
		SideEffect:      SideEffectWrite,
		FlatFields:      []string{"localPath", "remotePath"},
		FallbackExample: `{"action":"sftp_upload_file","arguments":{"localPath":"/tmp/report.txt","remotePath":"/tmp/report.txt"}}`,
		Parse:           parseSftpUploadFileTool,
	},
	{
		Name: ToolSftpDownload,
		Description: "Download a file from the authorized SSH host to a LOCAL path. This writes a local " +
			"file and always asks the user to approve the exact paths first. Refuses to overwrite an " +
			"existing local file: pick a different local path instead. At most 64 MiB per call.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"remotePath": map[string]any{
					"type":        "string",
					"description": "Absolute path of the file on the SSH host.",
				},
				"localPath": map[string]any{
					"type":        "string",
					"description": "Absolute destination path on the machine running Qterm. Must not exist yet.",
				},
			},
			"required": []string{"remotePath", "localPath"},
		},
		Kind:            StepKindSftpDownloadFile,
		SessionKinds:    []string{"ssh"},
		SideEffect:      SideEffectWrite,
		FlatFields:      []string{"remotePath", "localPath"},
		FallbackExample: `{"action":"sftp_download_file","arguments":{"remotePath":"/var/log/syslog","localPath":"/tmp/syslog"}}`,
		Parse:           parseSftpDownloadFileTool,
	},
	{
		Name: ToolLocalListDir,
		Description: "List a directory on the LOCAL machine running Qterm. Read-only and needs no approval. " +
			"Use it to find local files before reading them. Only available when the session is a local shell.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute local path, or a path starting with ~/ for the current user's home.",
				},
			},
			"required": []string{"path"},
		},
		Kind:            StepKindLocalListDir,
		SessionKinds:    []string{"local"},
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"path"},
		FallbackExample: `{"action":"local_list_dir","arguments":{"path":"/Users/me/project"}}`,
		Parse:           parseLocalListDirTool,
	},
	{
		Name: ToolLocalReadFile,
		Description: "Read a text file on the LOCAL machine running Qterm. Read-only and needs no approval. " +
			"Use it for local configuration or script files; binary and oversized files are reported instead of dumped. " +
			"Only available when the session is a local shell.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute local path, or a path starting with ~/ for the current user's home.",
				},
			},
			"required": []string{"path"},
		},
		Kind:            StepKindLocalReadFile,
		SessionKinds:    []string{"local"},
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"path"},
		FallbackExample: `{"action":"local_read_file","arguments":{"path":"~/notes.txt"}}`,
		Parse:           parseLocalReadFileTool,
	},
	{
		Name: ToolWebFetch,
		Description: "Fetch a web page and return its readable text. Read-only and needs no approval. " +
			"Only http/https is allowed; loopback and private addresses are blocked unless the user " +
			"enabled intranet access in the AI settings.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Absolute http/https URL to read.",
				},
			},
			"required": []string{"url"},
		},
		Kind:            StepKindWebFetch,
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"url"},
		FallbackExample: `{"action":"web_fetch","arguments":{"url":"https://example.com/docs"}}`,
		Parse:           parseWebFetchTool,
	},
	{
		Name: ToolAskUser,
		Description: "Ask the user a structured question with 2-6 options and wait for the answer. " +
			"Use it when a decision is genuinely ambiguous (which host, which file, which approach); " +
			"do not use it to ask for confirmation of an action you can already take.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "One short question for the user, in the user's language.",
				},
				"options": map[string]any{
					"type":        "array",
					"minItems":    2,
					"maxItems":    6,
					"items":       map[string]any{"type": "string"},
					"description": "Two to six mutually exclusive options; the user may also type a custom answer.",
				},
			},
			"required": []string{"question", "options"},
		},
		Kind:             StepKindAskUser,
		SideEffect:       SideEffectReadOnly,
		RequiresDecision: true,
		FlatFields:       []string{"question", "options"},
		FallbackExample:  `{"action":"ask_user","arguments":{"question":"要改哪台主机？","options":["web-1","db-1"]}}`,
		Parse:            parseAskUserTool,
	},
	{
		Name:        ToolFinalAnswer,
		Description: "Finish the task and answer the user. Call it when no more commands are needed.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"answer": map[string]any{
					"type":        "string",
					"description": "The complete answer for the user, in the user's language.",
				},
			},
			"required": []string{"answer"},
		},
		Kind:            StepKindAnswer,
		SideEffect:      SideEffectReadOnly,
		FlatFields:      []string{"answer"},
		FallbackExample: `{"action":"final_answer","arguments":{"answer":"..."}}`,
		Parse:           parseFinalAnswerTool,
	},
}

func AgentTools() []ToolSpec {
	specs := make([]ToolSpec, 0, len(agentToolDefs))
	for _, def := range agentToolDefs {
		specs = append(specs, ToolSpec{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.Parameters,
		})
	}
	return specs
}

func AgentToolsFor(sessionKind string) []ToolSpec {
	specs := make([]ToolSpec, 0, len(agentToolDefs))
	for _, def := range agentToolDefs {
		if !toolAppliesToSession(def, sessionKind) {
			continue
		}
		specs = append(specs, ToolSpec{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.Parameters,
		})
	}
	return specs
}

func toolAppliesToSession(def ToolDef, sessionKind string) bool {
	if len(def.SessionKinds) == 0 {
		return true
	}
	for _, kind := range def.SessionKinds {
		if kind == sessionKind {
			return true
		}
	}
	return false
}

func LookupTool(name string) (ToolDef, bool) {
	trimmed := strings.TrimSpace(name)
	for _, def := range agentToolDefs {
		if def.Name == trimmed {
			return def, true
		}
	}
	return ToolDef{}, false
}

func KnownStepKind(kind StepKind) bool {
	for _, def := range agentToolDefs {
		if def.Kind == kind {
			return true
		}
	}
	return false
}

func ToolSideEffectFor(kind StepKind) (ToolSideEffect, bool) {
	for _, def := range agentToolDefs {
		if def.Kind == kind {
			return def.SideEffect, true
		}
	}
	return "", false
}

func StepKindNeedsApproval(kind StepKind) bool {
	for _, def := range agentToolDefs {
		if def.Kind == kind {
			return def.SideEffect != SideEffectReadOnly || def.RequiresDecision
		}
	}
	return false
}

func toolArguments(call ToolCall) string {
	arguments := strings.TrimSpace(call.Arguments)
	if arguments == "" {
		return "{}"
	}
	return arguments
}

func toolArgumentPreview(arguments string) string {
	preview := redactAgentContext(arguments)
	if len([]rune(preview)) > 160 {
		preview = string([]rune(preview)[:160]) + "…"
	}
	return preview
}

func parseExecuteCommandTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Command         string `json:"command"`
		Description     string `json:"description"`
		TargetSessionID string `json:"target_session_id"`
		HostID          string `json:"host_id"`
		Risk            string `json:"risk"`
		RiskReason      string `json:"risk_reason"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("command arguments are invalid: " + preview)
	}
	command := strings.TrimSpace(payload.Command)
	if command == "" {
		return AgentAction{}, agentInvalidAction("command is required: " + preview)
	}
	if len(command) > maxAgentCommandBytes {
		return AgentAction{}, agentInvalidAction("command is too large")
	}
	description := strings.TrimSpace(payload.Description)
	if description == "" {
		description = firstLine(command)
	}
	if len([]rune(description)) > maxAgentDescriptionRune {
		description = string([]rune(description)[:maxAgentDescriptionRune])
	}

	risk := strings.TrimSpace(payload.Risk)
	if risk != RiskReadOnly && risk != RiskConfirm && risk != RiskDanger {
		risk = ""
	}
	return AgentAction{
		Kind:            StepKindCommand,
		CallID:          strings.TrimSpace(call.ID),
		Command:         command,
		HostID:          strings.TrimSpace(payload.HostID),
		TargetSessionID: strings.TrimSpace(payload.TargetSessionID),
		Description:     description,
		Risk:            risk,
		RiskReason:      strings.TrimSpace(payload.RiskReason),
	}, nil
}

func parseFinalAnswerTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Answer  string `json:"answer"`
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("answer arguments are invalid: " + preview)
	}
	answer := strings.TrimSpace(payload.Answer)
	if answer == "" {
		answer = strings.TrimSpace(payload.Content)
	}
	if answer == "" {
		answer = strings.TrimSpace(payload.Text)
	}
	if answer == "" {
		return AgentAction{}, agentInvalidAction("answer is required: " + preview)
	}
	if len(answer) > maxAgentAnswerBytes {
		return AgentAction{}, agentInvalidAction("answer is too large")
	}
	return AgentAction{Kind: StepKindAnswer, CallID: strings.TrimSpace(call.ID), Answer: answer}, nil
}

const (
	defaultAgentHostLimit = 20
	maxAgentHostLimit     = 50
	maxAgentHostQueryRune = 120
	maxAgentHostIDRune    = 128
)

func parseListHostsTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("list_hosts arguments are invalid: " + preview)
	}
	query := strings.TrimSpace(payload.Query)
	if len([]rune(query)) > maxAgentHostQueryRune {
		query = string([]rune(query)[:maxAgentHostQueryRune])
	}
	limit := payload.Limit
	if limit <= 0 {
		limit = defaultAgentHostLimit
	}
	if limit > maxAgentHostLimit {
		limit = maxAgentHostLimit
	}
	normalized, err := json.Marshal(struct {
		Query string `json:"query,omitempty"`
		Limit int    `json:"limit"`
	}{Query: query, Limit: limit})
	if err != nil {
		return AgentAction{}, agentInvalidAction("list_hosts arguments are invalid: " + preview)
	}
	return AgentAction{
		Kind:   StepKindListHosts,
		CallID: strings.TrimSpace(call.ID),
		Args:   normalized,
	}, nil
}

func parseGetHostTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		HostID string `json:"host_id"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("get_host arguments are invalid: " + preview)
	}
	hostID := strings.TrimSpace(payload.HostID)
	if hostID == "" {
		return AgentAction{}, agentInvalidAction("host_id is required: " + preview)
	}
	if len([]rune(hostID)) > maxAgentHostIDRune {
		return AgentAction{}, agentInvalidAction("host_id is too long")
	}
	normalized, err := json.Marshal(struct {
		HostID string `json:"host_id"`
	}{HostID: hostID})
	if err != nil {
		return AgentAction{}, agentInvalidAction("get_host arguments are invalid: " + preview)
	}
	return AgentAction{
		Kind:   StepKindGetHost,
		CallID: strings.TrimSpace(call.ID),
		Args:   normalized,
	}, nil
}

const maxAgentRemotePathBytes = 4096

func parseRemotePathArgument(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", agentInvalidAction("path is required")
	}
	if !strings.HasPrefix(path, "/") {
		return "", agentInvalidAction("path must be absolute (start with /)")
	}
	if len(path) > maxAgentRemotePathBytes {
		return "", agentInvalidAction("path is too long")
	}
	if strings.IndexByte(path, 0) >= 0 || !utf8.ValidString(path) {
		return "", agentInvalidAction("path is not a valid string")
	}
	return path, nil
}

func parseSftpListTool(call ToolCall) (AgentAction, error) {
	return parseSinglePathTool(call, ToolSftpList, StepKindSftpList)
}

func parseSftpReadFileTool(call ToolCall) (AgentAction, error) {
	return parseSinglePathTool(call, ToolSftpReadFile, StepKindSftpReadFile)
}

func parseSinglePathTool(call ToolCall, toolName string, kind StepKind) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	path, err := parseRemotePathArgument(payload.Path)
	if err != nil {
		return AgentAction{}, err
	}
	normalized, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: path})
	if err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	return AgentAction{
		Kind:   kind,
		CallID: strings.TrimSpace(call.ID),
		Args:   normalized,
	}, nil
}

func parseSftpUploadFileTool(call ToolCall) (AgentAction, error) {
	return parseSftpTransferTool(call, ToolSftpUploadFile, StepKindSftpUploadFile)
}

func parseSftpDownloadFileTool(call ToolCall) (AgentAction, error) {
	return parseSftpTransferTool(call, ToolSftpDownload, StepKindSftpDownloadFile)
}

func parseSftpTransferTool(call ToolCall, toolName string, kind StepKind) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		LocalPath  string `json:"localPath"`
		RemotePath string `json:"remotePath"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	localPath, err := parseLocalPathArgument(payload.LocalPath)
	if err != nil {
		return AgentAction{}, err
	}
	remotePath, err := parseRemotePathArgument(payload.RemotePath)
	if err != nil {
		return AgentAction{}, err
	}

	normalized, err := json.Marshal(struct {
		LocalPath  string `json:"localPath"`
		RemotePath string `json:"remotePath"`
	}{LocalPath: localPath, RemotePath: remotePath})
	if err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	return AgentAction{
		Kind:       kind,
		CallID:     strings.TrimSpace(call.ID),
		Args:       normalized,
		PublicArgs: normalized,
	}, nil
}

const maxAgentWriteBytes = 64 << 10

func parseSftpWriteFileTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("sftp_write_file arguments are invalid: " + preview)
	}
	path, err := parseRemotePathArgument(payload.Path)
	if err != nil {
		return AgentAction{}, err
	}
	content := payload.Content
	if len(content) > maxAgentWriteBytes {
		return AgentAction{}, agentInvalidAction("content is too large for one write (max 64 KiB)")
	}
	if !utf8.ValidString(content) {
		return AgentAction{}, agentInvalidAction("content is not valid UTF-8")
	}
	normalized, err := json.Marshal(struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}{Path: path, Content: content})
	if err != nil {
		return AgentAction{}, agentInvalidAction("sftp_write_file arguments are invalid: " + preview)
	}
	public, err := json.Marshal(struct {
		Path string `json:"path"`
		Size int    `json:"size"`
	}{Path: path, Size: len(content)})
	if err != nil {
		return AgentAction{}, agentInvalidAction("sftp_write_file arguments are invalid: " + preview)
	}
	return AgentAction{
		Kind:       StepKindSftpWriteFile,
		CallID:     strings.TrimSpace(call.ID),
		Args:       normalized,
		PublicArgs: public,
	}, nil
}

func parseLocalPathArgument(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", agentInvalidAction("path is required")
	}
	if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~/") {
		return "", agentInvalidAction("path must be absolute or start with ~/")
	}
	if len(path) > maxAgentRemotePathBytes {
		return "", agentInvalidAction("path is too long")
	}
	if strings.IndexByte(path, 0) >= 0 || !utf8.ValidString(path) {
		return "", agentInvalidAction("path is not a valid string")
	}
	return path, nil
}

func parseLocalListDirTool(call ToolCall) (AgentAction, error) {
	return parseLocalPathTool(call, ToolLocalListDir, StepKindLocalListDir)
}

func parseLocalReadFileTool(call ToolCall) (AgentAction, error) {
	return parseLocalPathTool(call, ToolLocalReadFile, StepKindLocalReadFile)
}

func parseLocalPathTool(call ToolCall, toolName string, kind StepKind) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	path, err := parseLocalPathArgument(payload.Path)
	if err != nil {
		return AgentAction{}, err
	}
	normalized, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: path})
	if err != nil {
		return AgentAction{}, agentInvalidAction(toolName + " arguments are invalid: " + preview)
	}
	return AgentAction{Kind: kind, CallID: strings.TrimSpace(call.ID), Args: normalized}, nil
}

const maxAgentURLBytes = 2048

func parseWebFetchTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("web_fetch arguments are invalid: " + preview)
	}
	raw := strings.TrimSpace(payload.URL)
	if raw == "" {
		return AgentAction{}, agentInvalidAction("url is required")
	}
	if len(raw) > maxAgentURLBytes {
		return AgentAction{}, agentInvalidAction("url is too long")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return AgentAction{}, agentInvalidAction("url must be an absolute http/https URL")
	}
	normalized, err := json.Marshal(struct {
		URL string `json:"url"`
	}{URL: raw})
	if err != nil {
		return AgentAction{}, agentInvalidAction("web_fetch arguments are invalid: " + preview)
	}
	return AgentAction{Kind: StepKindWebFetch, CallID: strings.TrimSpace(call.ID), Args: normalized}, nil
}

const (
	maxAgentQuestionRune = 500
	maxAgentOptionRune   = 120
	maxAgentOptions      = 6
	minAgentOptions      = 2

	maxAgentChoiceRune = 200
)

func parseAskUserTool(call ToolCall) (AgentAction, error) {
	arguments := toolArguments(call)
	preview := toolArgumentPreview(arguments)
	var payload struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return AgentAction{}, agentInvalidAction("ask_user arguments are invalid: " + preview)
	}
	question := strings.TrimSpace(payload.Question)
	if question == "" {
		return AgentAction{}, agentInvalidAction("question is required: " + preview)
	}
	if len([]rune(question)) > maxAgentQuestionRune {
		question = string([]rune(question)[:maxAgentQuestionRune])
	}
	if len(payload.Options) < minAgentOptions || len(payload.Options) > maxAgentOptions {
		return AgentAction{}, agentInvalidAction("ask_user needs 2-6 options")
	}
	options := make([]string, 0, len(payload.Options))
	seen := make(map[string]bool, len(payload.Options))
	for _, raw := range payload.Options {
		option := strings.TrimSpace(raw)
		if option == "" {
			return AgentAction{}, agentInvalidAction("option must not be empty")
		}
		if len([]rune(option)) > maxAgentOptionRune {
			option = string([]rune(option)[:maxAgentOptionRune])
		}
		if seen[option] {
			return AgentAction{}, agentInvalidAction("options must be unique")
		}
		seen[option] = true
		options = append(options, option)
	}
	normalized, err := json.Marshal(struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}{Question: question, Options: options})
	if err != nil {
		return AgentAction{}, agentInvalidAction("ask_user arguments are invalid: " + preview)
	}
	return AgentAction{Kind: StepKindAskUser, CallID: strings.TrimSpace(call.ID), Args: normalized}, nil
}
