package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	ToolExecuteCommand = "execute_command"
	ToolFinalAnswer    = "final_answer"
	ToolListHosts      = "list_hosts"
	ToolGetHost        = "get_host"
	ToolSftpList       = "sftp_list"
	ToolSftpReadFile   = "sftp_read_file"
	ToolSftpWriteFile  = "sftp_write_file"
	ToolSftpUploadFile = "sftp_upload_file"
	ToolSftpDownload   = "sftp_download_file"
	ToolLocalListDir   = "local_list_dir"
	ToolLocalReadFile  = "local_read_file"
	ToolWebFetch       = "web_fetch"
	ToolAskUser        = "ask_user"

	maxAgentCommandBytes    = 8 << 10
	maxAgentAnswerBytes     = 64 << 10
	maxAgentDescriptionRune = 200
)

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type AgentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	ToolCalls []ToolCall `json:"toolCalls,omitempty"`

	ToolCallID string `json:"toolCallId,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type AgentAction struct {
	Kind    StepKind
	CallID  string
	Command string

	HostID          string
	TargetSessionID string
	Description     string
	Risk            string
	RiskReason      string
	Answer          string

	Args json.RawMessage

	PublicArgs json.RawMessage
}

func AgentSystemPrompt(fallback bool, sessionKind, targetLabel string) string {
	var builder strings.Builder
	builder.WriteString("You are Qterm's desktop agent. You act only by calling tools; each turn calls exactly one tool from this list:\n")
	for _, def := range agentToolDefs {
		if !toolAppliesToSession(def, sessionKind) {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(def.Name)
		builder.WriteString(": ")
		builder.WriteString(def.Description)
		builder.WriteString("\n")
	}

	if label := strings.TrimSpace(targetLabel); label != "" {
		builder.WriteString("Target machine: ")
		builder.WriteString(label)
		builder.WriteString(". Every command and file operation runs on this machine only. " +
			"If you are unsure (long conversation, compacted history, user switches topic), call list_hosts " +
			"and read its `current` field instead of guessing.\n")
	} else {
		builder.WriteString("No terminal session is attached: you must pass host_id when calling execute_command.\n")
	}
	builder.WriteString("Rules:\n" +
		"1. Call exactly one tool per turn. Never emit two actions.\n" +
		"2. Never invent a target session; commands always run on the host the user authorized. " +
		"In global mode (no session attached) you must pass host_id to execute_command. An already-open " +
		"session is reused; if the host has none, the app connects with the saved host configuration " +
		"(a temporary connection recycled when idle), so inspect the machine instead of asking the user " +
		"to connect first. Connecting never widens the authorization scope, and hosts needing interactive " +
		"authentication will fail and must be connected by the user.\n" +
		"3. Commands are executed only after the user's configured approval; the backend takes the higher of your risk assessment and its own.\n" +
		"4. Read-only tools need no approval and never change anything.\n" +
		"5. Do not put commands in prose or code fences. Do not ask the user to copy output back.\n")
	if !fallback {
		return builder.String()
	}
	builder.WriteString("6. This provider does not support native tool calls. Respond with ONE JSON object and nothing else, using one of these shapes:\n")
	for _, def := range agentToolDefs {
		if !toolAppliesToSession(def, sessionKind) {
			continue
		}
		builder.WriteString(def.FallbackExample)
		builder.WriteString("\n")
	}
	builder.WriteString("No markdown fences, no commentary before or after the object.")
	return builder.String()
}

func SupportsNativeTools(providerType ProviderType) bool {
	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeAnthropic:
		return true
	default:
		return false
	}
}

func ParseToolCalls(calls []ToolCall) (AgentAction, error) {
	if len(calls) == 0 {
		return AgentAction{}, agentInvalidAction("model returned no action")
	}
	if len(calls) > 1 {
		return AgentAction{}, agentMultipleActions()
	}
	return parseToolCall(calls[0])
}

func parseToolCall(call ToolCall) (AgentAction, error) {
	def, ok := LookupTool(normalizeFallbackActionName(call.Name))
	if !ok {
		return AgentAction{}, agentUnknownTool()
	}
	action, err := def.Parse(call)
	if err != nil {
		return AgentAction{}, err
	}

	if action.Kind != def.Kind {
		return AgentAction{}, agentInvalidAction("tool returned an unexpected step kind")
	}
	return action, nil
}

func firstLine(command string) string {
	line := command
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	line = strings.TrimSpace(line)
	if len([]rune(line)) > 80 {
		line = string([]rune(line)[:80]) + "…"
	}
	return line
}

func ParseFallbackAction(content string) (AgentAction, error) {
	trimmed := stripSingleJSONFence(strings.TrimSpace(content))
	if trimmed == "" {
		return AgentAction{}, agentInvalidAction("model returned empty response")
	}
	if !strings.HasPrefix(trimmed, "{") {
		return AgentAction{}, agentInvalidAction("response is not a JSON object")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {

		repaired := repairJSONControlChars(trimmed)
		if repaired == trimmed || json.Unmarshal([]byte(repaired), &raw) != nil {
			return AgentAction{}, agentInvalidAction("response is not a valid JSON object")
		}
	}
	actionName := ""
	for _, key := range []string{"action", "tool", "name"} {
		if value, ok := raw[key]; ok {
			var name string
			if json.Unmarshal(value, &name) == nil && strings.TrimSpace(name) != "" {
				actionName = normalizeFallbackActionName(name)
				break
			}
		}
	}
	if actionName == "" {
		return AgentAction{}, agentInvalidAction("action is required")
	}
	arguments := json.RawMessage(nil)
	for _, key := range []string{"arguments", "parameters", "args", "input"} {
		if value, ok := raw[key]; ok && strings.TrimSpace(string(value)) != "" && string(value) != "null" {
			arguments = value
			break
		}
	}
	if arguments == nil {

		flat := make(map[string]json.RawMessage)
		def, known := LookupTool(actionName)
		for key, value := range raw {
			switch key {
			case "action", "tool", "name", "arguments", "parameters", "args", "input", "type":
				continue
			}
			if !known {
				continue
			}
			for _, field := range def.FlatFields {
				if key == field {
					flat[key] = value
					break
				}
			}
		}
		encoded, err := json.Marshal(flat)
		if err != nil {
			return AgentAction{}, agentInvalidAction("arguments are required")
		}
		arguments = encoded
	}
	return parseToolCall(ToolCall{Name: actionName, Arguments: string(arguments)})
}

func ExtractJSONAction(content string) (AgentAction, bool) {
	for index := 0; index < len(content); index++ {
		if content[index] != '{' {
			continue
		}
		end := balancedJSONEnd(content, index)
		if end < 0 {
			continue
		}
		candidate := content[index : end+1]
		index = end
		if !strings.Contains(candidate, `"action"`) && !strings.Contains(candidate, `"tool"`) && !strings.Contains(candidate, `"name"`) {
			continue
		}
		if action, err := ParseFallbackAction(candidate); err == nil {
			return action, true
		}
	}
	return AgentAction{}, false
}

func balancedJSONEnd(content string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(content); index++ {
		char := content[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func repairJSONControlChars(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	inString := false
	escaped := false
	for _, char := range value {
		if inString {
			if escaped {
				builder.WriteRune(char)
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
				builder.WriteRune(char)
			case '"':
				inString = false
				builder.WriteRune(char)
			case '\n':
				builder.WriteString(`\n`)
			case '\r':
				builder.WriteString(`\r`)
			case '\t':
				builder.WriteString(`\t`)
			default:
				if char < 0x20 {
					builder.WriteString(fmt.Sprintf(`\u%04x`, char))
				} else {
					builder.WriteRune(char)
				}
			}
			continue
		}
		if char == '"' {
			inString = true
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func stripSingleJSONFence(value string) string {
	if !strings.HasPrefix(value, "```") {
		return value
	}
	firstBreak := strings.IndexByte(value, '\n')
	if firstBreak < 0 || !strings.HasSuffix(value, "```") {
		return value
	}
	inner := strings.TrimSpace(value[firstBreak+1 : len(value)-3])
	if strings.HasPrefix(inner, "{") && strings.HasSuffix(inner, "}") {
		return inner
	}
	return value
}

func normalizeFallbackActionName(value string) string {
	switch strings.TrimSpace(value) {
	case ToolExecuteCommand, "executeCommand", "run_command", "runCommand":
		return ToolExecuteCommand
	case ToolFinalAnswer, "finalAnswer", "answer":
		return ToolFinalAnswer
	default:
		return strings.TrimSpace(value)
	}
}

func decodeStrict(value string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func ValidateActionTarget(action AgentAction, sessionID string) error {
	if action.Kind != StepKindCommand {
		return nil
	}
	if sessionID == "" {

		if action.TargetSessionID != "" {
			return fmt.Errorf("%w: global runs cannot target a session", ErrActionOutsideTarget)
		}
		if action.HostID == "" {
			return fmt.Errorf("%w: global runs must specify host_id", ErrActionOutsideTarget)
		}
		return nil
	}
	if action.TargetSessionID != "" && action.TargetSessionID != sessionID {
		return fmt.Errorf("%w: target session is outside the authorized run", ErrActionOutsideTarget)
	}
	return nil
}

var ErrActionOutsideTarget = errors.New("action target is outside the authorized session")

func salvageFinalAnswer(content string) (string, bool) {
	if !strings.Contains(content, "final_answer") {
		return "", false
	}
	marker := `"answer"`
	index := strings.Index(content, marker)
	if index < 0 {
		return "", false
	}
	rest := content[index+len(marker):]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", false
	}
	rest = strings.TrimSpace(rest[colon+1:])
	if !strings.HasPrefix(rest, `"`) {
		return "", false
	}
	rest = rest[1:]

	end := strings.LastIndex(rest, `"}`)
	if end < 0 {
		end = strings.LastIndex(rest, `"`)
	}
	if end < 0 {
		return "", false
	}
	raw := rest[:end]
	if unquoted, err := strconv.Unquote(`"` + raw + `"`); err == nil {
		if strings.TrimSpace(unquoted) != "" {
			return unquoted, true
		}
	}

	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", false
	}
	return cleaned, true
}
