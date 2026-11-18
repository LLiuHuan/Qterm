package model

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

const (
	MaxSSHAlgorithmCount = 32

	MaxSSHAlgorithmNameLength = 64

	MaxSSHProxyCommandLength = 4096
)

func sshHostKeyAlgorithmSet() map[string]struct{} {
	supported := ssh.SupportedAlgorithms()
	insecure := ssh.InsecureAlgorithms()
	set := make(map[string]struct{}, len(supported.HostKeys)+len(insecure.HostKeys))
	for _, name := range supported.HostKeys {
		set[name] = struct{}{}
	}
	for _, name := range insecure.HostKeys {
		set[name] = struct{}{}
	}
	return set
}

func sshKeyExchangeSet() map[string]struct{} {
	supported := ssh.SupportedAlgorithms()
	insecure := ssh.InsecureAlgorithms()
	set := make(map[string]struct{}, len(supported.KeyExchanges)+len(insecure.KeyExchanges))
	for _, name := range supported.KeyExchanges {
		set[name] = struct{}{}
	}
	for _, name := range insecure.KeyExchanges {
		set[name] = struct{}{}
	}
	return set
}

func SupportedSSHHostKeyAlgorithms() []string {
	return append([]string{}, ssh.SupportedAlgorithms().HostKeys...)
}

func DeprecatedSSHHostKeyAlgorithms() []string {
	return append([]string{}, ssh.InsecureAlgorithms().HostKeys...)
}

func SupportedSSHKeyExchanges() []string {
	return append([]string{}, ssh.SupportedAlgorithms().KeyExchanges...)
}

func DeprecatedSSHKeyExchanges() []string {
	return append([]string{}, ssh.InsecureAlgorithms().KeyExchanges...)
}

func NormalizeSSHAlgorithms(field string, values []string, allowed map[string]struct{}) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > MaxSSHAlgorithmCount {
		return nil, hostValidationError(field, "条目过多")
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len(name) > MaxSSHAlgorithmNameLength {
			return nil, hostValidationError(field, "算法名过长")
		}
		if _, ok := allowed[name]; !ok {
			return nil, hostValidationError(field, "不是本版本支持的算法："+name)
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func NormalizeSSHHostKeyAlgorithms(field string, values []string) ([]string, error) {
	return NormalizeSSHAlgorithms(field, values, sshHostKeyAlgorithmSet())
}

func NormalizeSSHKeyExchanges(field string, values []string) ([]string, error) {
	return NormalizeSSHAlgorithms(field, values, sshKeyExchangeSet())
}

func NormalizeSSHProxyCommand(value string) (string, error) {
	command := strings.TrimSpace(value)
	if command == "" {
		return "", nil
	}
	if len(command) > MaxSSHProxyCommandLength {
		return "", hostValidationError("sshProxyCommand", "命令过长")
	}
	if strings.IndexByte(command, 0) >= 0 {
		return "", hostValidationError("sshProxyCommand", "命令包含非法字符")
	}
	if !utf8.ValidString(command) {
		return "", hostValidationError("sshProxyCommand", "命令不是合法的 UTF-8 文本")
	}
	return command, nil
}

type SSHHandshakeAlgorithms struct {
	HostKeyAlgorithms []string `json:"hostKeyAlgorithms"`

	DeprecatedHostKeyAlgorithms []string `json:"deprecatedHostKeyAlgorithms"`

	KeyExchanges []string `json:"keyExchanges"`

	DeprecatedKeyExchanges []string `json:"deprecatedKeyExchanges"`
}

func NewSSHHandshakeAlgorithms() SSHHandshakeAlgorithms {
	return SSHHandshakeAlgorithms{
		HostKeyAlgorithms:           SupportedSSHHostKeyAlgorithms(),
		DeprecatedHostKeyAlgorithms: DeprecatedSSHHostKeyAlgorithms(),
		KeyExchanges:                SupportedSSHKeyExchanges(),
		DeprecatedKeyExchanges:      DeprecatedSSHKeyExchanges(),
	}
}
