package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/cliapi"
)

const defaultTimeout = 130 * time.Second

const (
	exitOK          = 0
	exitUsage       = 1
	exitConnect     = 2
	exitHandshake   = 3
	exitAuth        = 4
	exitBusiness    = 5
	maxResponseSize = 2 << 20
)

var (
	errConnect   = errors.New("cannot reach the Qterm app")
	errAuth      = errors.New("unauthorized")
	errHandshake = errors.New("handshake file is missing or invalid")
)

type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string {
	if e.message != "" {
		return fmt.Sprintf("%s: %s (HTTP %d)", e.code, e.message, e.status)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.code, e.status)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	options, command, rest, err := parseGlobal(args)
	if err != nil {
		fmt.Fprintln(stderr, "qterm:", err)
		printUsage(stderr)
		return exitUsage
	}
	if options.help {
		printUsage(stdout)
		return exitOK
	}
	if command == "" {
		printUsage(stderr)
		return exitUsage
	}

	handshake, err := cliapi.ReadHandshake(options.handshakePath)
	if err != nil {
		fmt.Fprintf(stderr, "qterm: %v: %v\n", errHandshake, err)
		return exitHandshake
	}
	client := &client{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", handshake.Port),
		token:   handshake.Token,
		http:    &http.Client{Timeout: options.timeout},
		jsonOut: options.json,
		stdout:  stdout,
		stderr:  stderr,
	}
	if err := dispatch(client, command, rest); err != nil {
		return exitCodeFor(err, stderr)
	}
	return exitOK
}

type globalOptions struct {
	handshakePath string
	json          bool
	timeout       time.Duration
	help          bool
}

func parseGlobal(args []string) (globalOptions, string, []string, error) {
	defaultPath, err := defaultHandshakePath()
	if err != nil {
		return globalOptions{}, "", nil, err
	}
	options := globalOptions{handshakePath: defaultPath, timeout: defaultTimeout}
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			break
		}
		switch {
		case arg == "--json":
			options.json = true
			index++
		case arg == "--help" || arg == "-h":
			options.help = true
			index++
		case arg == "--handshake":
			if index+1 >= len(args) {
				return options, "", nil, errors.New("--handshake requires a path")
			}
			options.handshakePath = args[index+1]
			index += 2
		case strings.HasPrefix(arg, "--handshake="):
			options.handshakePath = strings.TrimPrefix(arg, "--handshake=")
			index++
		case arg == "--timeout":
			if index+1 >= len(args) {
				return options, "", nil, errors.New("--timeout requires a duration")
			}
			parsed, parseErr := time.ParseDuration(args[index+1])
			if parseErr != nil || parsed <= 0 {
				return options, "", nil, fmt.Errorf("invalid --timeout %q", args[index+1])
			}
			options.timeout = parsed
			index += 2
		case strings.HasPrefix(arg, "--timeout="):
			parsed, parseErr := time.ParseDuration(strings.TrimPrefix(arg, "--timeout="))
			if parseErr != nil || parsed <= 0 {
				return options, "", nil, fmt.Errorf("invalid --timeout %q", arg)
			}
			options.timeout = parsed
			index++
		default:
			return options, "", nil, fmt.Errorf("unknown global flag %q", arg)
		}
	}
	if options.handshakePath == "" {
		return options, "", nil, errors.New("handshake path must not be empty")
	}
	if index >= len(args) {
		return options, "", nil, nil
	}
	return options, args[index], args[index+1:], nil
}

func defaultHandshakePath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(directory, "Qterm", "cli.json"), nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `qterm — Qterm 桌面端命令行客户端

用法:
  qterm [--handshake <path>] [--json] [--timeout 60s] <命令> [参数...]

命令:
  status                                    应用状态与计数
  hosts [--query <关键词>]                  列出资产（不返回任何凭据字段）
  sessions                                  列出会话
  open <hostID>                             打开会话
  close <sessionID>                         关闭会话
  exec <sessionID> [--] <命令...>           在 SSH 会话上执行一条后台命令
  sftp ls <sessionID> <path>                列出远端目录
  sftp read <sessionID> <path>              读取远端文本文件
  sftp write <sessionID> <path> --expected-hash <hash> [--content <text> | --content-file <file>]
                                            覆盖写入远端文本文件（哈希不匹配则冲突）
  shell <sessionID> [--cols N] [--rows N]   打开远端交互式终端（独立 PTY，不影响 GUI 终端）
  completion <bash|zsh|fish>                输出 shell 补全脚本（source 后生效）

退出码:
  0 成功 · 1 用法错误 · 2 连不上应用 · 3 握手文件缺失/损坏 · 4 鉴权失败 · 5 业务错误

说明:
  shell 的远端退出码会打印到 stderr，但命令本身仍以 0 退出（通道建立与关闭都成功）。
`)
}

func dispatch(c *client, command string, args []string) error {

	for len(args) > 0 && args[0] == "--json" {
		c.jsonOut = true
		args = args[1:]
	}
	switch command {
	case "status":
		return c.cmdStatus(args)
	case "hosts":
		return c.cmdHosts(args)
	case "sessions":
		return c.cmdSessions(args)
	case "open":
		return c.cmdOpen(args)
	case "close":
		return c.cmdClose(args)
	case "exec":
		return c.cmdExec(args)
	case "sftp":
		return c.cmdSftp(args)
	case "shell":
		return c.cmdShell(args)
	case "completion":
		return c.cmdCompletion(args)
	case "help":
		printUsage(c.stdout)
		return nil
	default:
		return usageError(fmt.Sprintf("unknown command %q", command))
	}
}

type usageError string

func (e usageError) Error() string { return string(e) }

func exitCodeFor(err error, stderr io.Writer) int {
	var usageErr usageError
	var businessErr *apiError
	switch {
	case errors.As(err, &usageErr):
		fmt.Fprintln(stderr, "qterm:", err)
		printUsage(stderr)
		return exitUsage
	case errors.Is(err, errHandshake):
		fmt.Fprintln(stderr, "qterm:", err)
		return exitHandshake
	case errors.Is(err, errAuth):
		fmt.Fprintln(stderr, "qterm: unauthorized: the handshake token is not accepted (restart the app or re-read the handshake file)")
		return exitAuth
	case errors.Is(err, errConnect):
		fmt.Fprintf(stderr, "qterm: %v: %v\n", errConnect, err)
		return exitConnect
	case errors.As(err, &businessErr):
		fmt.Fprintln(stderr, "qterm:", err)
		return exitBusiness
	default:
		fmt.Fprintln(stderr, "qterm:", err)
		return exitBusiness
	}
}

type client struct {
	baseURL string
	token   string
	http    *http.Client
	jsonOut bool
	stdout  io.Writer
	stderr  io.Writer
}

func (c *client) call(method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errConnect, err)
	}
	defer func() { _ = response.Body.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if readErr != nil {
		return nil, fmt.Errorf("%w: %v", errConnect, readErr)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errAuth
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var parsed struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &parsed) == nil && parsed.Error.Code != "" {
			return nil, &apiError{status: response.StatusCode, code: parsed.Error.Code, message: parsed.Error.Message}
		}
		return nil, &apiError{status: response.StatusCode, code: "HTTP_ERROR"}
	}
	return data, nil
}

func (c *client) print(raw []byte, human func()) {
	if c.jsonOut {
		_, _ = c.stdout.Write(bytes.TrimRight(raw, "\n"))
		_, _ = io.WriteString(c.stdout, "\n")
		return
	}
	human()
}

func (c *client) cmdStatus(args []string) error {
	if len(args) != 0 {
		return usageError("status: unexpected arguments")
	}
	raw, err := c.call(http.MethodGet, "/v1/status", nil)
	if err != nil {
		return err
	}
	var status cliapi.StatusResponse
	if err := json.Unmarshal(raw, &status); err != nil {
		return fmt.Errorf("decode status: %w", err)
	}
	c.print(raw, func() {
		fmt.Fprintf(c.stdout, "Qterm %s · pid %d · %d hosts · %d sessions\n",
			status.Version, status.PID, status.HostCount, status.SessionCount)
	})
	return nil
}

func (c *client) cmdHosts(args []string) error {
	fs := flag.NewFlagSet("hosts", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	query := fs.String("query", "", "case-insensitive keyword filter")
	if err := fs.Parse(args); err != nil {
		return usageError(fmt.Sprintf("hosts: %v", err))
	}
	if fs.NArg() != 0 {
		return usageError("hosts: unexpected arguments")
	}
	raw, err := c.call(http.MethodGet, "/v1/hosts", nil)
	if err != nil {
		return err
	}
	var hosts []cliapi.HostView
	if err := json.Unmarshal(raw, &hosts); err != nil {
		return fmt.Errorf("decode hosts: %w", err)
	}
	filtered := filterHosts(hosts, *query)
	c.print(raw, func() {
		if len(filtered) == 0 {
			fmt.Fprintln(c.stdout, "no hosts")
			return
		}
		rows := make([][]string, 0, len(filtered))
		for _, host := range filtered {
			rows = append(rows, []string{
				host.ID, host.CustomID, host.Name, host.Address, fmt.Sprint(host.Port), host.Protocol, host.Group, strings.Join(host.Tags, ","),
			})
		}

		writeTable(c.stdout, []string{"ID", "CUSTOM-ID", "NAME", "ADDRESS", "PORT", "PROTOCOL", "GROUP", "TAGS"}, rows)
	})
	return nil
}

func filterHosts(hosts []cliapi.HostView, query string) []cliapi.HostView {
	keyword := strings.ToLower(strings.TrimSpace(query))
	if keyword == "" {
		return hosts
	}
	filtered := make([]cliapi.HostView, 0, len(hosts))
	for _, host := range hosts {
		fields := []string{host.ID, host.CustomID, host.Name, host.Address, host.Group}
		fields = append(fields, host.Tags...)
		for _, field := range fields {
			if strings.Contains(strings.ToLower(field), keyword) {
				filtered = append(filtered, host)
				break
			}
		}
	}
	return filtered
}

func (c *client) cmdSessions(args []string) error {
	if len(args) != 0 {
		return usageError("sessions: unexpected arguments")
	}
	raw, err := c.call(http.MethodGet, "/v1/sessions", nil)
	if err != nil {
		return err
	}
	var sessions []cliapi.SessionView
	if err := json.Unmarshal(raw, &sessions); err != nil {
		return fmt.Errorf("decode sessions: %w", err)
	}
	c.print(raw, func() {
		if len(sessions) == 0 {
			fmt.Fprintln(c.stdout, "no sessions")
			return
		}
		rows := make([][]string, 0, len(sessions))
		for _, session := range sessions {
			rows = append(rows, []string{session.ID, session.HostID, session.Title, session.Kind, session.State})
		}
		writeTable(c.stdout, []string{"ID", "HOST", "TITLE", "KIND", "STATE"}, rows)
	})
	return nil
}

func (c *client) cmdOpen(args []string) error {
	if len(args) != 1 {
		return usageError("open: expects exactly one hostID")
	}
	raw, err := c.call(http.MethodPost, "/v1/sessions", map[string]string{"hostId": args[0]})
	if err != nil {
		return err
	}
	var session cliapi.SessionView
	if err := json.Unmarshal(raw, &session); err != nil {
		return fmt.Errorf("decode session: %w", err)
	}
	c.print(raw, func() {
		fmt.Fprintf(c.stdout, "session %s opened · %s %s · %s\n", session.ID, session.Kind, session.State, session.Title)
	})
	return nil
}

func (c *client) cmdClose(args []string) error {
	if len(args) != 1 {
		return usageError("close: expects exactly one sessionID")
	}
	raw, err := c.call(http.MethodDelete, "/v1/sessions/"+url.PathEscape(args[0]), nil)
	if err != nil {
		return err
	}
	c.print(raw, func() {
		fmt.Fprintf(c.stdout, "session %s closed\n", args[0])
	})
	return nil
}

func (c *client) cmdExec(args []string) error {
	if len(args) < 2 {
		return usageError("exec: expects <sessionID> and a command")
	}
	sessionID := args[0]
	rest := args[1:]

	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	command := strings.Join(rest, " ")
	if strings.TrimSpace(command) == "" {
		return usageError("exec: command must not be empty")
	}
	raw, err := c.call(http.MethodPost, "/v1/exec", map[string]string{"sessionId": sessionID, "command": command})
	if err != nil {
		return err
	}
	var result cliapi.ExecResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode exec: %w", err)
	}
	if c.jsonOut {
		c.print(raw, nil)
		return nil
	}
	_, _ = io.WriteString(c.stdout, result.Output)
	if !strings.HasSuffix(result.Output, "\n") && result.Output != "" {
		_, _ = io.WriteString(c.stdout, "\n")
	}
	if result.Truncated {
		fmt.Fprintln(c.stderr, "qterm: output truncated by the app")
	}
	if result.ExitCode != 0 {
		fmt.Fprintf(c.stderr, "qterm: remote exit code %d\n", result.ExitCode)
	}
	return nil
}

func (c *client) cmdCompletion(args []string) error {
	if len(args) != 1 {
		return usageError("completion 需要一个参数：bash、zsh 或 fish")
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(c.stdout, bashCompletionScript)
	case "zsh":
		fmt.Fprint(c.stdout, zshCompletionScript)
	case "fish":
		fmt.Fprint(c.stdout, fishCompletionScript)
	default:
		return usageError(fmt.Sprintf("completion 不支持 %q（支持 bash、zsh、fish）", args[0]))
	}
	return nil
}

const qtermSubcommands = "status hosts sessions open close exec sftp shell completion help"

const bashCompletionScript = `# qterm bash 补全：source <(qterm completion bash)
_qterm_complete() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  case "$prev" in
    completion) COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") ); return 0 ;;
    sftp) COMPREPLY=( $(compgen -W "ls read write" -- "$cur") ); return 0 ;;
  esac
  if [ "$COMP_CWORD" -le 1 ]; then
    COMPREPLY=( $(compgen -W "` + qtermSubcommands + `" -- "$cur") )
    return 0
  fi
  COMPREPLY=( $(compgen -f -- "$cur") )
}
complete -F _qterm_complete qterm
`

const zshCompletionScript = `#compdef qterm
# qterm zsh 补全：source <(qterm completion zsh)
_qterm() {
  local -a commands
  commands=(` + qtermSubcommands + `)
  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi
  case "${words[2]}" in
    completion) _values 'shell' bash zsh fish ;;
    sftp) _values 'action' ls read write ;;
    *) _files ;;
  esac
}
_qterm "$@"
`

const fishCompletionScript = `# qterm fish 补全：qterm completion fish | source
complete -c qterm -f -n '__fish_use_subcommand' -a '` + qtermSubcommands + `'
complete -c qterm -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c qterm -f -n '__fish_seen_subcommand_from sftp' -a 'ls read write'
`

func (c *client) cmdSftp(args []string) error {
	if len(args) == 0 {
		return usageError("sftp: expects a subcommand (ls|read|write)")
	}
	switch args[0] {
	case "ls":
		return c.cmdSftpList(args[1:])
	case "read":
		return c.cmdSftpRead(args[1:])
	case "write":
		return c.cmdSftpWrite(args[1:])
	default:
		return usageError(fmt.Sprintf("sftp: unknown subcommand %q", args[0]))
	}
}

func (c *client) cmdSftpList(args []string) error {
	if len(args) != 2 {
		return usageError("sftp ls: expects <sessionID> <path>")
	}
	query := url.Values{"sessionId": {args[0]}, "path": {args[1]}}
	raw, err := c.call(http.MethodGet, "/v1/sftp/list?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	var result cliapi.SftpListResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode list: %w", err)
	}
	c.print(raw, func() {
		if len(result.Entries) == 0 {
			fmt.Fprintf(c.stdout, "%s: empty directory\n", result.Path)
			return
		}
		rows := make([][]string, 0, len(result.Entries))
		for _, entry := range result.Entries {
			rows = append(rows, []string{entry.Name, entry.Type, fmt.Sprint(entry.Size), entry.ModTime})
		}
		writeTable(c.stdout, []string{"NAME", "TYPE", "SIZE", "MODIFIED"}, rows)
		if result.Truncated {
			fmt.Fprintln(c.stderr, "qterm: entry list truncated by the app")
		}
	})
	return nil
}

func (c *client) cmdSftpRead(args []string) error {
	if len(args) != 2 {
		return usageError("sftp read: expects <sessionID> <path>")
	}
	query := url.Values{"sessionId": {args[0]}, "path": {args[1]}}
	raw, err := c.call(http.MethodGet, "/v1/sftp/read?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	var result cliapi.SftpReadResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode read: %w", err)
	}
	if c.jsonOut {
		c.print(raw, nil)
		return nil
	}
	_, _ = io.WriteString(c.stdout, result.Content)
	if !strings.HasSuffix(result.Content, "\n") && result.Content != "" {
		_, _ = io.WriteString(c.stdout, "\n")
	}
	if result.Truncated {
		fmt.Fprintln(c.stderr, "qterm: file content truncated by the app")
	}
	if result.Hash != "" {
		fmt.Fprintf(c.stderr, "qterm: hash %s · %d bytes\n", result.Hash, result.Size)
	}
	return nil
}

func (c *client) cmdSftpWrite(args []string) error {

	if len(args) < 2 {
		return usageError("sftp write: expects <sessionID> <path> and options")
	}
	sessionID, path := args[0], args[1]
	fs := flag.NewFlagSet("sftp write", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	expectedHash := fs.String("expected-hash", "", "expected current file hash (empty means create)")
	content := fs.String("content", "", "inline content")
	contentFile := fs.String("content-file", "", "read content from a local file")
	if err := fs.Parse(args[2:]); err != nil {
		return usageError(fmt.Sprintf("sftp write: %v", err))
	}
	if fs.NArg() != 0 {
		return usageError("sftp write: unexpected arguments")
	}

	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	if provided["content"] && provided["content-file"] {
		return usageError("sftp write: --content and --content-file are mutually exclusive")
	}
	if !provided["content"] && !provided["content-file"] {
		return usageError("sftp write: one of --content or --content-file is required")
	}
	payload := *content
	if provided["content-file"] {
		data, err := os.ReadFile(*contentFile)
		if err != nil {
			return fmt.Errorf("read content file: %w", err)
		}
		payload = string(data)
	}
	raw, err := c.call(http.MethodPut, "/v1/sftp/write", map[string]string{
		"sessionId":    sessionID,
		"path":         path,
		"content":      payload,
		"expectedHash": *expectedHash,
	})
	if err != nil {
		return err
	}
	var result cliapi.SftpWriteResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode write: %w", err)
	}
	if c.jsonOut {
		c.print(raw, nil)
		if result.Conflict {
			fmt.Fprintf(c.stderr, "qterm: write conflict, current hash %s\n", result.CurrentHash)
			return &apiError{status: http.StatusConflict, code: "CONFLICT", message: "file changed since the expected hash"}
		}
		return nil
	}
	if result.Conflict {

		fmt.Fprintf(c.stderr, "qterm: write conflict, current hash %s (nothing was written)\n", result.CurrentHash)
		return &apiError{status: http.StatusConflict, code: "CONFLICT", message: "file changed since the expected hash"}
	}
	fmt.Fprintf(c.stdout, "saved %s\n", path)
	return nil
}

func writeTable(w io.Writer, header []string, rows [][]string) {
	widths := make([]int, len(header))
	for index, cell := range header {
		widths[index] = len(cell)
	}
	for _, row := range rows {
		for index, cell := range row {
			if index < len(widths) && len(cell) > widths[index] {
				widths[index] = len(cell)
			}
		}
	}
	printRow := func(cells []string) {
		var builder strings.Builder
		for index, cell := range cells {
			if index > 0 {
				builder.WriteString("  ")
			}
			builder.WriteString(cell)
			if index < len(cells)-1 {
				builder.WriteString(strings.Repeat(" ", widths[index]-len(cell)))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(builder.String(), " "))
	}
	printRow(header)
	for _, row := range rows {
		printRow(row)
	}
}
