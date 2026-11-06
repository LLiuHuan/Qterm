package hostimport

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"changeme/internal/model"
)

var ErrConfigNotFound = errors.New("ssh config not found")

type Entry struct {
	Alias string `json:"alias"`

	HostName string `json:"hostName"`
	User     string `json:"user"`
	Port     int    `json:"port"`

	IdentityFile string `json:"identityFile,omitempty"`

	Line int `json:"line"`

	Protocol string `json:"protocol,omitempty"`

	SerialBaud     int    `json:"serialBaud,omitempty"`
	SerialDataBits int    `json:"serialDataBits,omitempty"`
	SerialParity   string `json:"serialParity,omitempty"`
	SerialStopBits string `json:"serialStopBits,omitempty"`

	Secret string `json:"secret,omitempty"`

	CredentialID string `json:"credentialId,omitempty"`

	Group string `json:"group,omitempty"`

	Password string `json:"password,omitempty"`

	PasswordSource string `json:"passwordSource,omitempty"`

	Duplicate bool `json:"duplicate"`
}

type Result struct {
	Entries []Entry `json:"entries"`

	Skipped int `json:"skipped"`
}

func DefaultSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func ParseSSHConfigFile(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("open ssh config: %w", err)
	}
	defer file.Close()
	return ParseSSHConfig(file)
}

func ParseSSHConfig(reader interface{ Read([]byte) (int, error) }) (Result, error) {
	result := Result{Entries: make([]Entry, 0, 8)}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNumber := 0
	current := (*Entry)(nil)
	haveHostName := false

	flush := func() {
		if current == nil {
			return
		}

		if current.HostName == "" {
			current.HostName = current.Alias
		}
		if current.Port == 0 {
			current.Port = 22
		}
		result.Entries = append(result.Entries, *current)
		current = nil
		haveHostName = false
	}

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value := splitDirective(line)
		switch strings.ToLower(key) {
		case "host":
			flush()
			alias, ok := pickAlias(value)
			if !ok {

				result.Skipped++
				continue
			}
			current = &Entry{Alias: alias, Port: 22, Line: lineNumber}
		case "match":

			if current != nil {
				result.Skipped++
				current = nil
				haveHostName = false
			}
		case "hostname":
			if current != nil && value != "" {
				current.HostName = value
				haveHostName = true
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "port":
			if current != nil {
				if port, err := strconv.Atoi(value); err == nil && port > 0 && port <= 65535 {
					current.Port = port
				}
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = shortenHome(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("read ssh config: %w", err)
	}
	flush()
	_ = haveHostName
	return result, nil
}

func splitDirective(line string) (string, string) {
	if index := strings.IndexAny(line, " \t="); index >= 0 {
		key := strings.TrimSpace(line[:index])
		value := strings.TrimSpace(strings.TrimLeft(line[index:], " \t="))
		return key, value
	}
	return line, ""
}

func pickAlias(value string) (string, bool) {
	fields := strings.Fields(value)
	if len(fields) != 1 {
		return "", false
	}
	alias := fields[0]
	if strings.ContainsAny(alias, "*?!") {
		return "", false
	}
	return alias, true
}

func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

func (e Entry) HostInput() model.HostInput {
	protocol := strings.TrimSpace(e.Protocol)
	if protocol == "" {
		protocol = model.ProtocolSSH
	}
	input := model.HostInput{
		Name:     e.Alias,
		Protocol: protocol,
		Address:  e.HostName,
		Port:     e.Port,
		Username: e.User,
		Group:    e.Group,
	}

	if protocol == model.ProtocolSerial {

		input.Port = 0
		if e.SerialBaud > 0 {
			baud := e.SerialBaud
			input.SerialBaud = &baud
		}
		if e.SerialDataBits > 0 {
			bits := e.SerialDataBits
			input.SerialDataBits = &bits
		}
		if e.SerialParity != "" {
			parity := e.SerialParity
			input.SerialParity = &parity
		}
		if e.SerialStopBits != "" {
			stopBits := e.SerialStopBits
			input.SerialStopBits = &stopBits
		}
		return input
	}
	if e.CredentialID != "" {
		input.CredentialID = e.CredentialID
	} else if password := firstNonEmpty(e.Secret, e.Password); password != "" {
		input.Password = password
		input.PasswordSource = "saved"
	} else if e.PasswordSource != "" {

		input.PasswordSource = e.PasswordSource
	}
	return input
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
