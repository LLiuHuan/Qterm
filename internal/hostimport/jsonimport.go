package hostimport

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type nyatermJSON struct {
	Version   int             `json:"version"`
	Passwords []nyatermSecret `json:"passwords"`
	SSHKeys   []nyatermSecret `json:"ssh_keys"`
	Groups    []struct {
		Path []string `json:"path"`
	} `json:"groups"`
	Sessions []nyatermSession `json:"sessions"`
}

type nyatermSecret struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type nyatermSession struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	GroupPath []string `json:"group_path"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Username  string   `json:"username"`
	PortName  string   `json:"port_name"`
	BaudRate  int      `json:"baud_rate"`
	DataBits  int      `json:"data_bits"`
	Parity    string   `json:"parity"`
	StopBits  string   `json:"stop_bits"`
	Auth      struct {
		Mode        string `json:"mode"`
		Password    string `json:"password"`
		PasswordRef string `json:"password_ref"`
		KeyRef      string `json:"key_ref"`
	} `json:"auth"`
}

func ParseNyaTermJSON(data []byte) (Result, error) {
	var document nyatermJSON
	if err := json.Unmarshal(data, &document); err != nil {

		return ParseTermiusJSON(data)
	}
	if document.Sessions == nil {
		return ParseTermiusJSON(data)
	}
	passwords := make(map[string]string, len(document.Passwords))
	for _, item := range document.Passwords {
		if item.Ref != "" && item.Password != "" {
			passwords[item.Ref] = item.Password
		}
	}
	result := Result{Entries: make([]Entry, 0, len(document.Sessions))}
	line := 0
	for _, session := range document.Sessions {
		line++
		entry, ok := nyatermSessionEntry(session, passwords)
		if !ok {
			result.Skipped++
			continue
		}
		entry.Line = line
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func ParseNyaTermJSONFile(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read json import: %w", err)
	}
	return ParseNyaTermJSON(data)
}

func nyatermSessionEntry(session nyatermSession, passwords map[string]string) (Entry, bool) {
	protocol := strings.ToLower(strings.TrimSpace(session.Type))
	group := strings.Join(session.GroupPath, "/")
	entry := Entry{Alias: strings.TrimSpace(session.Name), User: strings.TrimSpace(session.Username), Group: group}
	switch protocol {
	case "ssh", "":
		if session.Host == "" {
			return Entry{}, false
		}
		entry.Protocol = "ssh"
		entry.HostName = session.Host
		entry.Port = session.Port
		if entry.Port == 0 {
			entry.Port = 22
		}
		entry.Password = nyatermSessionPassword(session, passwords)
	case "telnet":
		if session.Host == "" {
			return Entry{}, false
		}
		entry.Protocol = "telnet"
		entry.HostName = session.Host
		entry.Port = session.Port
		if entry.Port == 0 {
			entry.Port = 23
		}
	case "serial":
		if session.PortName == "" {
			return Entry{}, false
		}
		entry.Protocol = "serial"
		entry.HostName = session.PortName
		entry.Port = 0
		entry.SerialBaud = session.BaudRate
		entry.SerialDataBits = session.DataBits
		entry.SerialParity = session.Parity
		entry.SerialStopBits = session.StopBits
	case "local_terminal":

		return Entry{}, false
	default:
		return Entry{}, false
	}
	if entry.Alias == "" {
		entry.Alias = entry.HostName
	}
	return entry, true
}

func nyatermSessionPassword(session nyatermSession, passwords map[string]string) string {
	if password := strings.TrimSpace(session.Auth.Password); password != "" {
		return password
	}
	if ref := strings.TrimSpace(session.Auth.PasswordRef); ref != "" {
		return passwords[ref]
	}
	return ""
}
