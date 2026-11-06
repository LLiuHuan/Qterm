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
	Kind            StepKind
	CallID          string
	Command         string
	TargetSessionID string
	Description     string
	Risk            string
	RiskReason      string
	Answer          string
}

func AgentTools() []ToolSpec {
	return []ToolSpec{
		{
			Name: ToolExecuteCommand,
			Description: "Run one shell command on the authorized terminal session. " +
				"Use it when you need information from the machine. Never wrap the command in a code fence.",
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
		},
	}
}

func AgentSystemPrompt(fallback bool) string {
	head := "You are Qterm's terminal agent. You can only act by calling exactly one of two tools per turn: " +
		ToolExecuteCommand + " (run one shell command on the authorized session) or " +
		ToolFinalAnswer + " (finish and answer the user).\n" +
		"Rules:\n" +
		"1. Call exactly one tool per turn. Never emit two actions.\n" +
		"2. Never invent a target session; commands always run on the session the user authorized.\n" +
		"3. Commands are executed only after the user's configured approval; the backend takes the higher of your risk assessment and its own.\n" +
		"4. Do not put commands in prose or code fences. Do not ask the user to copy output back.\n"
	if !fallback {
		return head
	}
	return head + "5. This provider does not support native tool calls. Respond with ONE JSON object and nothing else:\n" +
		`{"action":"execute_command","arguments":{"command":"df -h","description":"check disk usage","risk":"readonly"}}` + "\n" +
		`or {"action":"final_answer","arguments":{"answer":"..."}}` + "\n" +
		"No markdown fences, no commentary before or after the object."
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
	name := normalizeFallbackActionName(call.Name)
	if name != ToolExecuteCommand && name != ToolFinalAnswer {
		return AgentAction{}, agentUnknownTool()
	}
	arguments := strings.TrimSpace(call.Arguments)
	if arguments == "" {
		arguments = "{}"
	}
	preview := redactAgentContext(arguments)
	if len([]rune(preview)) > 160 {
		preview = string([]rune(preview)[:160]) + "…"
	}
	switch name {
	case ToolExecuteCommand:

		var payload struct {
			Command         string `json:"command"`
			Description     string `json:"description"`
			TargetSessionID string `json:"target_session_id"`
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
			TargetSessionID: strings.TrimSpace(payload.TargetSessionID),
			Description:     description,
			Risk:            risk,
			RiskReason:      strings.TrimSpace(payload.RiskReason),
		}, nil
	case ToolFinalAnswer:
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
	default:
		return AgentAction{}, agentUnknownTool()
	}
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
		for key, value := range raw {
			switch key {
			case "action", "tool", "name", "arguments", "parameters", "args", "input", "type":
				continue
			}
			switch actionName {
			case ToolExecuteCommand:
				switch key {
				case "command", "description", "target_session_id", "risk", "risk_reason":
					flat[key] = value
				}
			case ToolFinalAnswer:
				if key == "answer" {
					flat[key] = value
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
