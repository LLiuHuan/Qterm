package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	maxHistoryFileBytes = 1 << 20

	maxHistoryCommandBytes = 8 << 10

	maxHistoryEntries = 1000

	historyScannerBytes = maxHistoryCommandBytes * 2
)

const (
	msgHistoryHomeUnknown     = "history.home_unknown"
	msgHistoryReadUnsupported = "history.read_unsupported"
	msgHistoryUnreadable      = "history.unreadable"
)

type RemoteHistoryEntry struct {
	Shell string `json:"shell"`

	Command string `json:"command"`

	Timestamp int64 `json:"timestamp"`

	Source string `json:"source"`
}

type RemoteHistory struct {
	Entries []RemoteHistoryEntry `json:"entries"`

	Truncated bool `json:"truncated"`

	Home string `json:"home,omitempty"`
}

type remoteHistoryFileAccess interface {
	StatContext(ctx context.Context, remotePath string) (fs.FileInfo, error)
	OpenFile(ctx context.Context, remotePath string) (io.ReadCloser, error)
}

type remoteHistoryHomeResolver interface {
	WorkingDirectory(ctx context.Context) (string, error)
}

type historySource struct {
	shell  string
	source string

	relative string
}

var remoteHistorySources = []historySource{
	{shell: "bash", source: "~/.bash_history", relative: ".bash_history"},
	{shell: "zsh", source: "~/.zsh_history", relative: ".zsh_history"},
	{shell: "fish", source: "~/.local/share/fish/fish_history", relative: ".local/share/fish/fish_history"},
}

func (s *Service) ListRemoteHistory(ctx context.Context, sessionID string) (RemoteHistory, error) {
	if s == nil || s.sessions == nil {
		return RemoteHistory{}, errno.New(errno.CodeStoreFailed, "error.serviceNotReady", "")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return RemoteHistory{}, errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager, err := s.sessions.Manager()
	if err != nil {
		return RemoteHistory{}, publicError(err)
	}
	fileSystem, err := manager.FileSystem(ctx, sessionID)
	if err != nil {
		return RemoteHistory{}, publicError(err)
	}
	defer func() { _ = fileSystem.Close() }()

	access, ok := fileSystem.(remoteHistoryFileAccess)
	if !ok {
		return RemoteHistory{}, errno.New(errno.CodeInvalidArg, msgHistoryReadUnsupported, "")
	}
	home := ""
	if resolver, ok := fileSystem.(remoteHistoryHomeResolver); ok {
		if resolved, resolveErr := resolver.WorkingDirectory(ctx); resolveErr == nil {
			home = strings.TrimSpace(resolved)
		}
	}
	if home == "" {
		return RemoteHistory{}, errno.New(errno.CodeInvalidArg, msgHistoryHomeUnknown, "")
	}
	if normalized, normalizeErr := model.NormalizeRemoteDirectory(home); normalizeErr == nil {
		home = normalized
	}

	parsed := make([]remoteHistoryParse, 0, len(remoteHistorySources))
	unreadable := 0

	for _, source := range remoteHistorySources {
		remotePath := path.Join(home, source.relative)
		data, readTruncated, readErr := readHistoryFile(ctx, access, remotePath)
		if readErr != nil {
			if isHistoryNotExist(readErr) {
				continue
			}

			unreadable++
			continue
		}
		if len(data) == 0 {
			continue
		}
		entries := parseRemoteHistory(source, data)
		truncated := readTruncated
		if len(entries) > maxHistoryEntries {
			entries = entries[len(entries)-maxHistoryEntries:]
			truncated = true
		}
		parsed = append(parsed, remoteHistoryParse{entries: entries, truncated: truncated})
	}

	if len(parsed) == 0 {
		if unreadable > 0 {
			return RemoteHistory{}, errno.New(errno.CodePermissionDenied, msgHistoryUnreadable, "")
		}
		return RemoteHistory{Home: home}, nil
	}

	entries, truncated := combineRemoteHistory(parsed)
	return RemoteHistory{Entries: entries, Truncated: truncated, Home: home}, nil
}

type remoteHistoryParse struct {
	entries   []RemoteHistoryEntry
	truncated bool
}

func combineRemoteHistory(files []remoteHistoryParse) ([]RemoteHistoryEntry, bool) {
	if len(files) == 0 {
		return nil, false
	}
	budget := maxHistoryEntries / len(files)
	if budget < 1 {
		budget = 1
	}
	truncated := false
	entries := make([]RemoteHistoryEntry, 0, maxHistoryEntries)
	for _, item := range files {
		kept := item.entries
		if len(kept) > budget {
			kept = kept[len(kept)-budget:]
			truncated = true
		}
		truncated = truncated || item.truncated
		entries = append(entries, kept...)
	}
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
		truncated = true
	}

	sort.SliceStable(entries, func(left, right int) bool {
		return entries[left].Timestamp < entries[right].Timestamp
	})
	return entries, truncated
}

func readHistoryFile(
	ctx context.Context,
	access remoteHistoryFileAccess,
	remotePath string,
) ([]byte, bool, error) {
	info, err := access.StatContext(ctx, remotePath)
	if err != nil {
		return nil, false, err
	}
	if info == nil || !info.Mode().IsRegular() {
		return nil, false, nil
	}
	if info.Size() <= 0 {
		return nil, false, nil
	}
	reader, err := access.OpenFile(ctx, remotePath)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = reader.Close() }()

	size := info.Size()
	if size <= maxHistoryFileBytes {
		data, readErr := io.ReadAll(io.LimitReader(reader, maxHistoryFileBytes))
		return data, false, readErr
	}
	if seeker, ok := reader.(io.Seeker); ok {
		if _, seekErr := seeker.Seek(size-maxHistoryFileBytes, io.SeekStart); seekErr == nil {
			data, readErr := io.ReadAll(io.LimitReader(reader, maxHistoryFileBytes))
			if readErr != nil {
				return nil, true, readErr
			}

			if index := bytes.IndexByte(data, '\n'); index >= 0 {
				data = data[index+1:]
			} else {
				data = nil
			}
			return data, true, nil
		}

		if _, seekErr := seeker.Seek(0, io.SeekStart); seekErr != nil {
			return nil, true, seekErr
		}
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxHistoryFileBytes))
	return data, true, readErr
}

func isHistoryNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such file")
}

func scanLines(data []byte, handle func(line string)) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), historyScannerBytes)
	for scanner.Scan() {
		handle(scanner.Text())
	}

}

func parseRemoteHistory(source historySource, data []byte) []RemoteHistoryEntry {
	switch source.shell {
	case "zsh":
		return parseZshHistory(source, data)
	case "fish":
		return parseFishHistory(source, data)
	default:
		return parseBashHistory(source, data)
	}
}

func truncateCommand(value string) string {
	if len(value) <= maxHistoryCommandBytes {
		return value
	}

	runes := []rune(value)
	var builder strings.Builder
	for _, char := range runes {
		if builder.Len()+len(string(char)) > maxHistoryCommandBytes {
			break
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func stripTerminalSequences(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); {
		rest := value[index:]
		switch {
		case strings.HasPrefix(rest, "\x1b"):
			index += skipEscapeSequence(rest)
		case value[index] == 0x07:
			index++

		case value[index] < 0x20 && value[index] != '\t' && value[index] != '\n':
			index++
		case value[index] == 0x7f:
			index++
		default:
			builder.WriteByte(value[index])
			index++
		}
	}
	return builder.String()
}

func skipEscapeSequence(rest string) int {
	if len(rest) < 2 {
		return 1
	}
	switch rest[1] {
	case '[':
		for index := 2; index < len(rest); index++ {
			if rest[index] >= 0x40 && rest[index] <= 0x7e {
				return index + 1
			}
		}
		return len(rest)
	case ']':
		for index := 2; index < len(rest); index++ {
			if rest[index] == 0x07 {
				return index + 1
			}
			if rest[index] == 0x1b && index+1 < len(rest) && rest[index+1] == '\\' {
				return index + 2
			}
		}
		return len(rest)
	case 'P', '^', '_':
		for index := 2; index < len(rest); index++ {
			if rest[index] == 0x1b && index+1 < len(rest) && rest[index+1] == '\\' {
				return index + 2
			}
		}
		return len(rest)
	default:
		return 2
	}
}

func isTerminalReplyNoise(command string) bool {
	visible := strings.TrimSpace(stripTerminalSequences(command))
	if visible == "" {
		return true
	}

	if strings.HasPrefix(visible, ">|") {
		return true
	}

	if matched, _ := regexp.MatchString(`^\??[0-9;]*c$`, visible); matched {
		return true
	}

	if matched, _ := regexp.MatchString(`^[0-9]+;[0-9]+R$`, visible); matched {
		return true
	}
	return false
}

func sanitizeHistoryCommand(value string) string {

	cleaned := strings.TrimSpace(stripTerminalSequences(strings.ToValidUTF8(value, "\uFFFD")))
	return truncateCommand(cleaned)
}

func parseBashHistory(source historySource, data []byte) []RemoteHistoryEntry {
	entries := make([]RemoteHistoryEntry, 0, 64)
	var current strings.Builder
	var timestamp int64
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		if command := sanitizeHistoryCommand(current.String()); command != "" && !isTerminalReplyNoise(command) {
			entries = append(entries, RemoteHistoryEntry{
				Shell:     source.shell,
				Command:   command,
				Timestamp: timestamp,
				Source:    source.source,
			})
		}
		current.Reset()
		hasCurrent = false
		timestamp = 0
	}

	scanLines(data, func(line string) {
		trimmed := strings.TrimRight(line, "\r")

		if !hasCurrent && strings.HasPrefix(trimmed, "#") {
			if epoch, parseErr := strconv.ParseInt(strings.TrimSpace(trimmed[1:]), 10, 64); parseErr == nil {
				timestamp = epoch
				return
			}
		}
		if hasCurrent {
			current.WriteString("\n")
		}
		if strings.HasSuffix(trimmed, "\\") && !strings.HasSuffix(trimmed, "\\\\") {
			current.WriteString(strings.TrimSuffix(trimmed, "\\"))
			hasCurrent = true
			return
		}
		current.WriteString(trimmed)
		hasCurrent = true
		flush()
	})
	flush()
	return entries
}

func parseZshHistory(source historySource, data []byte) []RemoteHistoryEntry {
	entries := make([]RemoteHistoryEntry, 0, 64)
	var current strings.Builder
	var timestamp int64
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		if command := sanitizeHistoryCommand(current.String()); command != "" && !isTerminalReplyNoise(command) {
			entries = append(entries, RemoteHistoryEntry{
				Shell:     source.shell,
				Command:   command,
				Timestamp: timestamp,
				Source:    source.source,
			})
		}
		current.Reset()
		hasCurrent = false
		timestamp = 0
	}

	scanLines(data, func(line string) {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, ": ") {
			flush()
			epoch, command := parseZshMeta(trimmed)
			timestamp = epoch
			current.WriteString(command)
			hasCurrent = true
			return
		}
		if hasCurrent {
			current.WriteString("\n")
			current.WriteString(trimmed)
			return
		}
		if strings.TrimSpace(trimmed) == "" {
			return
		}

		current.WriteString(trimmed)
		hasCurrent = true
		flush()
	})
	flush()
	return entries
}

func parseZshMeta(line string) (int64, string) {
	body := strings.TrimPrefix(line, ": ")
	colon := strings.IndexByte(body, ':')
	if colon < 0 {
		return 0, line
	}
	epoch, parseErr := strconv.ParseInt(strings.TrimSpace(body[:colon]), 10, 64)
	if parseErr != nil {
		return 0, line
	}
	rest := body[colon+1:]
	semicolon := strings.IndexByte(rest, ';')
	if semicolon < 0 {
		return epoch, rest
	}
	return epoch, rest[semicolon+1:]
}

func parseFishHistory(source historySource, data []byte) []RemoteHistoryEntry {
	entries := make([]RemoteHistoryEntry, 0, 64)
	var current strings.Builder
	var timestamp int64
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		if command := sanitizeHistoryCommand(current.String()); command != "" && !isTerminalReplyNoise(command) {
			entries = append(entries, RemoteHistoryEntry{
				Shell:     source.shell,
				Command:   command,
				Timestamp: timestamp,
				Source:    source.source,
			})
		}
		current.Reset()
		hasCurrent = false
		timestamp = 0
	}

	scanLines(data, func(line string) {
		trimmed := strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(trimmed, "- cmd: "); ok {
			flush()
			current.WriteString(unescapeFishCommand(rest))
			hasCurrent = true
			return
		}
		if rest, ok := strings.CutPrefix(trimmed, "  when: "); ok && hasCurrent {
			if epoch, parseErr := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); parseErr == nil {
				timestamp = epoch
			}
			return
		}

		if hasCurrent && strings.HasPrefix(trimmed, "    ") {
			current.WriteString("\n")
			current.WriteString(strings.TrimSpace(trimmed))
		}
	})
	flush()
	return entries
}

func unescapeFishCommand(value string) string {
	if !strings.Contains(value, "\\") {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char != '\\' || index+1 >= len(value) {
			builder.WriteByte(char)
			continue
		}
		index++
		switch value[index] {
		case 'n':
			builder.WriteByte('\n')
		case '\\':
			builder.WriteByte('\\')
		default:
			builder.WriteByte('\\')
			builder.WriteByte(value[index])
		}
	}
	return builder.String()
}
