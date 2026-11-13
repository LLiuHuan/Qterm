package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxCredentialIDSize  = 1024
	maxHostNotesSize     = 4096
	maxSSHOTPEntryIDSize = 1024
)

var (
	ErrInvalidHost = errors.New("invalid host")

	ErrInvalidGroup = errors.New("invalid group")

	ErrInvalidCredential = errors.New("invalid credential")

	ErrCredentialVaultLocked = errors.New("本地认证信息当前不可用")

	ErrInvalidCredentialPromptRule = errors.New("invalid credential prompt rule")
)

type ValidationError struct {
	Field  string
	Reason string
	Err    error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

const (
	ProtocolSSH = "ssh"

	ProtocolTelnet = "telnet"

	ProtocolSerial = "serial"

	ProtocolRDP = "rdp"

	ProtocolVNC = "vnc"
)

const (
	defaultSSHPort    = 22
	defaultTelnetPort = 23
	defaultRDPPort    = 3389
	defaultVNCPort    = 5900
)

type HostInput struct {
	Color          string `json:"color,omitempty"`
	Icon           string `json:"icon"`
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Group          string `json:"group"`
	CredentialID   string `json:"credentialId"`
	PasswordSource string `json:"passwordSource,omitempty"`
	Notes          string `json:"notes"`

	SSHTwoFactorEnabled *bool `json:"sshTwoFactorEnabled,omitempty"`

	SSHOTPEntryID *string `json:"sshOtpEntryId,omitempty"`

	SSHOTPAutoFill   *bool  `json:"sshOtpAutoFill,omitempty"`
	Password         string `json:"password,omitempty"`
	ClearPassword    bool   `json:"clearPassword,omitempty"`
	SSHKeepAlive     *bool  `json:"sshKeepAlive,omitempty"`
	LatencyDetection *bool  `json:"latencyDetection,omitempty"`
	AutoDetectOSIcon *bool  `json:"autoDetectOSIcon,omitempty"`
	DirectoryFollow  string `json:"directoryFollow,omitempty"`

	SFTPShowHiddenFiles *string `json:"sftpShowHiddenFiles,omitempty"`

	SFTPDefaultDirectory *string `json:"sftpDefaultDirectory,omitempty"`

	BackspaceMode *string `json:"backspaceMode,omitempty"`

	Encoding *string `json:"encoding,omitempty"`

	ConnectTimeoutSeconds *int `json:"connectTimeoutSeconds,omitempty"`

	PostLoginCommand *string `json:"postLoginCommand,omitempty"`

	PostLoginCommandDelaySeconds *int `json:"postLoginCommandDelaySeconds,omitempty"`

	EnvironmentVariables []HostEnvironmentVariable `json:"environmentVariables,omitempty"`

	X11Forwarding *bool `json:"x11Forwarding,omitempty"`

	X11Trusted *bool `json:"x11Trusted,omitempty"`

	SerialBaud *int `json:"serialBaud,omitempty"`

	SerialDataBits *int `json:"serialDataBits,omitempty"`

	SerialParity *string `json:"serialParity,omitempty"`

	SerialStopBits *string `json:"serialStopBits,omitempty"`

	RdpDisplayMode *string `json:"rdpDisplayMode,omitempty"`

	RdpWidth  *int `json:"rdpWidth,omitempty"`
	RdpHeight *int `json:"rdpHeight,omitempty"`

	RdpClipboardMode *string `json:"rdpClipboardMode,omitempty"`

	RdpClipboardContent *string `json:"rdpClipboardContent,omitempty"`

	VncViewOnly *bool `json:"vncViewOnly,omitempty"`

	VncShared *bool `json:"vncShared,omitempty"`

	VncPixelDepth *int `json:"vncPixelDepth,omitempty"`

	VncScale *string `json:"vncScale,omitempty"`

	VncAutoReconnect *bool `json:"vncAutoReconnect,omitempty"`

	VncAutoReconnectAttempts *int `json:"vncAutoReconnectAttempts,omitempty"`

	VncTransport *string `json:"vncTransport,omitempty"`

	VncWsPath *string `json:"vncWsPath,omitempty"`

	VncAllowInsecureTLS *bool `json:"vncAllowInsecureTls,omitempty"`

	VncRfbPasswordMode *string `json:"vncRfbPasswordMode,omitempty"`

	Tags []string `json:"tags,omitempty"`

	ConnectionType *string `json:"connectionType,omitempty"`

	TerminalType *string `json:"terminalType,omitempty"`

	JumpHostID *string `json:"jumpHostId,omitempty"`

	ProxyID string `json:"proxyId,omitempty"`

	ProxyMode *string `json:"proxyMode,omitempty"`

	ProxyHost *string `json:"proxyHost,omitempty"`

	ProxyPort *int `json:"proxyPort,omitempty"`

	ProxyUsername *string `json:"proxyUsername,omitempty"`

	ProxyPassword      string `json:"proxyPassword,omitempty"`
	ClearProxyPassword bool   `json:"clearProxyPassword,omitempty"`
}

type GroupInput struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
}

type HostEnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Host struct {
	ID   string `json:"id"`
	Icon string `json:"icon"`

	Color              string `json:"color,omitempty"`
	Name               string `json:"name"`
	Protocol           string `json:"protocol"`
	Address            string `json:"address"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	GroupID            string `json:"groupId"`
	SortOrder          int    `json:"sortOrder"`
	Group              string `json:"group"`
	CredentialID       string `json:"credentialId"`
	PasswordSource     string `json:"passwordSource"`
	PasswordConfigured bool   `json:"passwordConfigured"`

	SSHTwoFactorEnabled bool `json:"sshTwoFactorEnabled"`

	SSHOTPEntryID string `json:"sshOtpEntryId,omitempty"`

	SSHOTPAutoFill bool `json:"sshOtpAutoFill"`

	ProxyPasswordConfigured bool      `json:"proxyPasswordConfigured"`
	Notes                   string    `json:"notes"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
	SSHKeepAlive            bool      `json:"sshKeepAlive"`
	LatencyDetection        bool      `json:"latencyDetection"`
	AutoDetectOSIcon        bool      `json:"autoDetectOSIcon"`
	DirectoryFollow         string    `json:"directoryFollow"`

	SFTPShowHiddenFiles string `json:"sftpShowHiddenFiles"`

	SFTPDefaultDirectory string `json:"sftpDefaultDirectory"`

	SerialBaud int `json:"serialBaud,omitempty"`

	SerialDataBits int `json:"serialDataBits,omitempty"`

	SerialParity string `json:"serialParity,omitempty"`

	SerialStopBits string `json:"serialStopBits,omitempty"`

	BackspaceMode                string `json:"backspaceMode"`
	Encoding                     string `json:"encoding"`
	ConnectTimeoutSeconds        int    `json:"connectTimeoutSeconds"`
	PostLoginCommand             string `json:"postLoginCommand"`
	PostLoginCommandDelaySeconds int    `json:"postLoginCommandDelaySeconds"`

	EnvironmentVariables []HostEnvironmentVariable `json:"environmentVariables"`

	X11Forwarding bool `json:"x11Forwarding"`
	X11Trusted    bool `json:"x11Trusted"`

	RdpDisplayMode string `json:"rdpDisplayMode"`
	RdpWidth       int    `json:"rdpWidth"`
	RdpHeight      int    `json:"rdpHeight"`

	RdpClipboardMode    string `json:"rdpClipboardMode"`
	RdpClipboardContent string `json:"rdpClipboardContent"`

	VncViewOnly              bool   `json:"vncViewOnly"`
	VncShared                bool   `json:"vncShared"`
	VncPixelDepth            int    `json:"vncPixelDepth"`
	VncScale                 string `json:"vncScale"`
	VncAutoReconnect         bool   `json:"vncAutoReconnect"`
	VncAutoReconnectAttempts int    `json:"vncAutoReconnectAttempts"`
	VncTransport             string `json:"vncTransport"`
	VncWsPath                string `json:"vncWsPath"`
	VncAllowInsecureTLS      bool   `json:"vncAllowInsecureTls"`

	VncRfbPasswordMode string   `json:"vncRfbPasswordMode"`
	Tags               []string `json:"tags"`

	ConnectionType string `json:"connectionType"`
	TerminalType   string `json:"terminalType"`

	JumpHostID *string `json:"jumpHostId,omitempty"`

	ProxyID       string  `json:"proxyId,omitempty"`
	ProxyMode     *string `json:"proxyMode,omitempty"`
	ProxyHost     *string `json:"proxyHost,omitempty"`
	ProxyPort     *int    `json:"proxyPort,omitempty"`
	ProxyUsername *string `json:"proxyUsername,omitempty"`
}

type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentId,omitempty"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func normalizeEnvironmentVariables(values []HostEnvironmentVariable) ([]HostEnvironmentVariable, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maxEnvironmentVariables {
		return nil, hostValidationError("environmentVariables", "数量过多")
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]HostEnvironmentVariable, 0, len(values))
	for _, item := range values {
		name := strings.TrimSpace(item.Name)
		value := item.Value
		if name == "" && strings.TrimSpace(value) == "" {

			continue
		}
		if !validEnvironmentVariableName(name) {
			return nil, hostValidationError("environmentVariables", "变量名不合法")
		}
		if len(name) > maxEnvironmentVariableNameLength || len(value) > maxEnvironmentVariableValueLength {
			return nil, hostValidationError("environmentVariables", "变量过长")
		}
		if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
			return nil, hostValidationError("environmentVariables", "变量值格式无效")
		}
		if _, exists := seen[name]; exists {
			return nil, hostValidationError("environmentVariables", "变量名重复")
		}
		seen[name] = struct{}{}
		normalized = append(normalized, HostEnvironmentVariable{Name: name, Value: value})
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func validEnvironmentVariableName(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		isLetter := char == '_' || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := char >= '0' && char <= '9'
		if index == 0 {
			if !isLetter {
				return false
			}
			continue
		}
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

var terminalTypes = []string{"vt100", "vt200", "xterm", "xterm-256color", "ansi", "linux"}

func ValidTerminalType(value string) bool {
	for _, item := range terminalTypes {
		if item == value {
			return true
		}
	}
	return false
}

func DefaultTerminalType(connectionType string) string {
	if connectionType == ConnectionTypeNetwork {
		return "vt100"
	}
	return "xterm-256color"
}

func sftpDefaultDirectoryValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

const (
	maxPostLoginCommandLength = 4096

	maxEnvironmentVariables = 64

	maxEnvironmentVariableNameLength  = 128
	maxEnvironmentVariableValueLength = 4096

	ConnectTimeoutSecondsMin = 1
	ConnectTimeoutSecondsMax = 300

	ConnectionTypeStandard = "standard"
	ConnectionTypeNetwork  = "network"

	MaxPostLoginCommandDelaySeconds = 300

	DefaultConnectTimeoutSeconds = 15

	VncScaleFit    = "fit"
	VncScaleActual = "actual"

	VncTransportTCP = "tcp"
	VncTransportWSS = "wss"

	VncRfbPasswordAuto = "auto"
	VncRfbPasswordSame = "same"

	DefaultVncWsPath = "/"

	MaxVncWsPathLength = 256

	DefaultVncAutoReconnectAttempts = 5
	MaxVncAutoReconnectAttempts     = 100

	ProxyModeNone   = "none"
	ProxyModeSOCKS5 = "socks5"
	ProxyModeHTTP   = "http"

	MaxJumpHostIDLength = 1024

	MaxProxyIDLength = 1024
)

func ValidProxyMode(mode string) bool {
	switch mode {
	case ProxyModeNone, ProxyModeSOCKS5, ProxyModeHTTP:
		return true
	default:
		return false
	}
}

func optionalTrimmedText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validOptionalHostText(value *string) bool {
	if value == nil {
		return true
	}
	return utf8.ValidString(*value) && strings.IndexByte(*value, 0) < 0
}

func NormalizeVncWsPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultVncWsPath, nil
	}
	if len(path) > MaxVncWsPathLength {
		return "", hostValidationError("vncWsPath", "超出长度上限")
	}
	if !strings.HasPrefix(path, "/") {
		return "", hostValidationError("vncWsPath", "必须以 / 开头")
	}

	if strings.HasPrefix(path, "//") {
		return "", hostValidationError("vncWsPath", "不能是协议相对地址")
	}
	if strings.Contains(path, "://") {
		return "", hostValidationError("vncWsPath", "不能包含协议前缀")
	}

	if strings.ContainsAny(path, " \t\r\n\v\f") {
		return "", hostValidationError("vncWsPath", "不能包含空白字符")
	}
	if strings.Contains(path, "..") {
		return "", hostValidationError("vncWsPath", "不能包含 ..")
	}
	return path, nil
}

func ValidateVncWsPath(path string) error {
	_, err := NormalizeVncWsPath(path)
	return err
}

const maxSFTPDefaultDirectoryLength = 1024

func NewHost(id string, input HostInput, now time.Time) (Host, error) {
	id = strings.TrimSpace(id)
	input.Name = strings.TrimSpace(input.Name)
	input.Icon = strings.TrimSpace(input.Icon)
	input.Color = strings.TrimSpace(input.Color)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.Address = strings.TrimSpace(input.Address)
	input.Username = strings.TrimSpace(input.Username)
	input.Group = strings.TrimSpace(input.Group)
	input.CredentialID = strings.TrimSpace(input.CredentialID)
	input.PasswordSource = strings.TrimSpace(input.PasswordSource)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.SSHKeepAlive == nil {
		value := true
		input.SSHKeepAlive = &value
	}
	if input.LatencyDetection == nil {
		value := true
		input.LatencyDetection = &value
	}
	if input.AutoDetectOSIcon == nil {
		value := true
		input.AutoDetectOSIcon = &value
	}
	if input.DirectoryFollow == "" {
		input.DirectoryFollow = "inherit"
	}
	if input.DirectoryFollow != "inherit" && input.DirectoryFollow != "enabled" && input.DirectoryFollow != "disabled" {
		return Host{}, hostValidationError("directoryFollow", "必须是 inherit、enabled 或 disabled")
	}

	showHidden := "inherit"
	if input.SFTPShowHiddenFiles != nil {
		if trimmed := strings.TrimSpace(*input.SFTPShowHiddenFiles); trimmed != "" {
			showHidden = trimmed
		}
	}
	input.SFTPShowHiddenFiles = &showHidden
	if *input.SFTPShowHiddenFiles != "inherit" && *input.SFTPShowHiddenFiles != "enabled" && *input.SFTPShowHiddenFiles != "disabled" {
		return Host{}, hostValidationError("sftpShowHiddenFiles", "必须是 inherit、enabled 或 disabled")
	}
	if input.SFTPDefaultDirectory != nil {
		value := strings.TrimSpace(*input.SFTPDefaultDirectory)
		input.SFTPDefaultDirectory = &value
	}
	if input.SFTPDefaultDirectory != nil && *input.SFTPDefaultDirectory != "" {
		directory := *input.SFTPDefaultDirectory

		if !strings.HasPrefix(directory, "/") {
			return Host{}, hostValidationError("sftpDefaultDirectory", "必须是远端绝对路径")
		}
		if len(directory) > maxSFTPDefaultDirectoryLength {
			return Host{}, hostValidationError("sftpDefaultDirectory", "路径过长")
		}
		for _, char := range directory {
			if char < 0x20 || char == 0x7f {
				return Host{}, hostValidationError("sftpDefaultDirectory", "路径包含控制字符")
			}
		}
	}
	connectionType := ConnectionTypeStandard
	if input.ConnectionType != nil {
		if trimmed := strings.TrimSpace(*input.ConnectionType); trimmed != "" {
			connectionType = trimmed
		}
	}
	if connectionType != ConnectionTypeStandard && connectionType != ConnectionTypeNetwork {
		return Host{}, hostValidationError("connectionType", "必须是 standard 或 network")
	}
	input.ConnectionType = &connectionType

	terminalType := ""
	if input.TerminalType != nil {
		terminalType = strings.TrimSpace(*input.TerminalType)
	}
	if terminalType == "" {
		terminalType = DefaultTerminalType(connectionType)
	}
	if !ValidTerminalType(terminalType) {
		return Host{}, hostValidationError("terminalType", "不是受支持的终端类型")
	}
	input.TerminalType = &terminalType

	jumpHostID := optionalTrimmedText(input.JumpHostID)
	if jumpHostID != nil {
		if len(*jumpHostID) > MaxJumpHostIDLength ||
			!utf8.ValidString(*jumpHostID) ||
			strings.IndexByte(*jumpHostID, 0) >= 0 {
			return Host{}, hostValidationError("jumpHostId", "格式无效")
		}
	}
	input.JumpHostID = jumpHostID

	proxyID := strings.TrimSpace(input.ProxyID)
	if len(proxyID) > MaxProxyIDLength ||
		!utf8.ValidString(proxyID) ||
		strings.IndexByte(proxyID, 0) >= 0 {
		return Host{}, hostValidationError("proxyId", "格式无效")
	}
	input.ProxyID = proxyID

	proxyMode := ProxyModeNone
	if input.ProxyMode != nil {
		if candidate := strings.TrimSpace(*input.ProxyMode); ValidProxyMode(candidate) {
			proxyMode = candidate
		}
	}
	input.ProxyMode = &proxyMode

	proxyHost := optionalTrimmedText(input.ProxyHost)
	proxyUsername := optionalTrimmedText(input.ProxyUsername)
	proxyPassword := input.ProxyPassword
	proxyPort := 0
	if input.ProxyPort != nil {
		proxyPort = *input.ProxyPort
	}
	if proxyMode == ProxyModeNone {

		proxyHost, proxyUsername, proxyPassword, proxyPort = nil, nil, "", 0
		input.ClearProxyPassword = true
	} else if proxyID != "" && (proxyHost == nil || proxyPort < 1 || proxyPort > 65535) {

		proxyHost, proxyUsername, proxyPassword, proxyPort = nil, nil, "", 0
		input.ClearProxyPassword = true
	} else {

		if proxyHost == nil {
			return Host{}, hostValidationError("proxyHost", "不能为空")
		}
		if !validOptionalHostText(proxyHost) {
			return Host{}, hostValidationError("proxyHost", "格式无效")
		}
		if !validOptionalHostText(proxyUsername) {
			return Host{}, hostValidationError("proxyUsername", "格式无效")
		}
		if proxyPassword != "" && proxyUsername == nil {
			return Host{}, hostValidationError("proxyUsername", "配置代理密码时不能为空")
		}
		if proxyPort < 1 || proxyPort > 65535 {
			return Host{}, hostValidationError("proxyPort", "必须在 1 到 65535 之间")
		}
	}
	input.ProxyHost = proxyHost
	input.ProxyUsername = proxyUsername
	input.ProxyPassword = proxyPassword
	if proxyPort == 0 {

		input.ProxyPort = nil
	} else {
		input.ProxyPort = &proxyPort
	}

	if input.X11Forwarding == nil {
		value := false
		input.X11Forwarding = &value
	}
	if input.X11Trusted == nil {

		value := true
		input.X11Trusted = &value
	}

	displayMode := "fit"
	if input.RdpDisplayMode != nil {
		if trimmed := strings.TrimSpace(*input.RdpDisplayMode); trimmed != "" {
			displayMode = trimmed
		}
	}
	if displayMode != "fit" && displayMode != "fixed" {
		return Host{}, hostValidationError("rdpDisplayMode", "不是受支持的远程桌面显示方式")
	}
	input.RdpDisplayMode = &displayMode
	if input.RdpWidth == nil {
		value := 1280
		input.RdpWidth = &value
	}
	if input.RdpHeight == nil {
		value := 800
		input.RdpHeight = &value
	}
	if *input.RdpWidth < 640 || *input.RdpWidth > 7680 || *input.RdpHeight < 480 || *input.RdpHeight > 4320 {
		return Host{}, hostValidationError("rdpWidth", "远程桌面尺寸超出允许范围（640×480 ~ 7680×4320）")
	}

	clipboardMode := "both"
	if input.RdpClipboardMode != nil {
		if trimmed := strings.TrimSpace(*input.RdpClipboardMode); trimmed != "" {
			clipboardMode = trimmed
		}
	}
	switch clipboardMode {
	case "off", "both", "local-to-remote", "remote-to-local":
	default:
		return Host{}, hostValidationError("rdpClipboardMode", "不是受支持的剪贴板方向")
	}
	input.RdpClipboardMode = &clipboardMode
	clipboardContent := "text"
	if input.RdpClipboardContent != nil {
		if trimmed := strings.TrimSpace(*input.RdpClipboardContent); trimmed != "" {
			clipboardContent = trimmed
		}
	}
	switch clipboardContent {
	case "text", "text-and-files":
	default:
		return Host{}, hostValidationError("rdpClipboardContent", "不是受支持的剪贴板内容")
	}
	input.RdpClipboardContent = &clipboardContent

	viewOnly := false
	if input.VncViewOnly != nil {
		viewOnly = *input.VncViewOnly
	}
	input.VncViewOnly = &viewOnly
	shared := true
	if input.VncShared != nil {
		shared = *input.VncShared
	}
	input.VncShared = &shared
	pixelDepth := 32
	if input.VncPixelDepth != nil {
		pixelDepth = *input.VncPixelDepth
	}
	switch pixelDepth {
	case 16, 32:
	default:
		return Host{}, hostValidationError("vncPixelDepth", "只支持 32 位或 16 位色深")
	}
	input.VncPixelDepth = &pixelDepth

	scale := VncScaleFit
	if input.VncScale != nil {
		if trimmed := strings.TrimSpace(*input.VncScale); trimmed != "" {
			scale = trimmed
		}
	}
	switch scale {
	case VncScaleFit, VncScaleActual:
	default:
		return Host{}, hostValidationError("vncScale", "只支持 fit 或 actual")
	}
	input.VncScale = &scale

	autoReconnect := false
	if input.VncAutoReconnect != nil {
		autoReconnect = *input.VncAutoReconnect
	}
	input.VncAutoReconnect = &autoReconnect

	reconnectAttempts := DefaultVncAutoReconnectAttempts
	if input.VncAutoReconnectAttempts != nil {
		reconnectAttempts = *input.VncAutoReconnectAttempts
	}
	if reconnectAttempts < 0 || reconnectAttempts > MaxVncAutoReconnectAttempts {
		return Host{}, hostValidationError("vncAutoReconnectAttempts", "超出允许范围")
	}
	input.VncAutoReconnectAttempts = &reconnectAttempts

	transport := VncTransportTCP
	if input.VncTransport != nil {
		if trimmed := strings.TrimSpace(*input.VncTransport); trimmed != "" {
			transport = trimmed
		}
	}
	switch transport {
	case VncTransportTCP, VncTransportWSS:
	default:
		return Host{}, hostValidationError("vncTransport", "只支持 tcp 或 wss")
	}
	input.VncTransport = &transport

	rawWsPath := ""
	if input.VncWsPath != nil {
		rawWsPath = *input.VncWsPath
	}
	wsPath, err := NormalizeVncWsPath(rawWsPath)
	if err != nil {
		return Host{}, err
	}
	input.VncWsPath = &wsPath

	allowInsecureTLS := false
	if input.VncAllowInsecureTLS != nil {
		allowInsecureTLS = *input.VncAllowInsecureTLS
	}
	input.VncAllowInsecureTLS = &allowInsecureTLS

	rfbPasswordMode := VncRfbPasswordAuto
	if input.VncRfbPasswordMode != nil {
		if trimmed := strings.TrimSpace(*input.VncRfbPasswordMode); trimmed != "" {
			rfbPasswordMode = trimmed
		}
	}
	switch rfbPasswordMode {
	case VncRfbPasswordAuto, VncRfbPasswordSame:
	default:
		return Host{}, hostValidationError("vncRfbPasswordMode", "只支持 auto 或 same")
	}
	input.VncRfbPasswordMode = &rfbPasswordMode

	tags, err := NormalizeHostTags(input.Tags)
	if err != nil {
		return Host{}, err
	}
	input.Tags = tags
	backspaceMode := "del"
	if input.BackspaceMode != nil {
		if trimmed := strings.TrimSpace(*input.BackspaceMode); trimmed != "" {
			backspaceMode = trimmed
		}
	}
	if backspaceMode != "del" && backspaceMode != "bs" {
		return Host{}, hostValidationError("backspaceMode", "必须是 del 或 bs")
	}
	input.BackspaceMode = &backspaceMode

	encoding := "utf-8"
	if input.Encoding != nil {
		if trimmed := strings.TrimSpace(*input.Encoding); trimmed != "" {
			encoding = trimmed
		}
	}
	if encoding != "utf-8" && encoding != "gbk" {
		return Host{}, hostValidationError("encoding", "必须是 utf-8 或 gbk")
	}
	input.Encoding = &encoding

	timeout := DefaultConnectTimeoutSeconds
	if input.ConnectTimeoutSeconds != nil && *input.ConnectTimeoutSeconds != 0 {
		timeout = *input.ConnectTimeoutSeconds
	}
	if timeout < ConnectTimeoutSecondsMin || timeout > ConnectTimeoutSecondsMax {
		return Host{}, hostValidationError("connectTimeoutSeconds", "超出允许范围")
	}
	input.ConnectTimeoutSeconds = &timeout

	postLoginCommand := ""
	if input.PostLoginCommand != nil {
		postLoginCommand = strings.TrimSpace(*input.PostLoginCommand)
	}
	if len(postLoginCommand) > maxPostLoginCommandLength ||
		!utf8.ValidString(postLoginCommand) ||
		strings.IndexByte(postLoginCommand, 0) >= 0 {
		return Host{}, hostValidationError("postLoginCommand", "格式无效")
	}
	input.PostLoginCommand = &postLoginCommand

	delaySeconds := 0
	if input.PostLoginCommandDelaySeconds != nil {
		delaySeconds = *input.PostLoginCommandDelaySeconds
	}
	if delaySeconds < 0 || delaySeconds > MaxPostLoginCommandDelaySeconds {
		return Host{}, hostValidationError("postLoginCommandDelaySeconds", "超出允许范围")
	}
	input.PostLoginCommandDelaySeconds = &delaySeconds

	environment, err := normalizeEnvironmentVariables(input.EnvironmentVariables)
	if err != nil {
		return Host{}, err
	}
	input.EnvironmentVariables = environment

	if input.PasswordSource == "" {
		if input.CredentialID != "" {
			input.PasswordSource = ""
		} else if input.Password != "" {
			input.PasswordSource = "saved"
		} else {
			input.PasswordSource = "none"
		}
	}
	if input.PasswordSource != "" && input.PasswordSource != "prompt" && input.PasswordSource != "saved" && input.PasswordSource != "none" {
		return Host{}, hostValidationError("passwordSource", "必须是 prompt、saved 或 none")
	}
	if input.CredentialID != "" {

		input.PasswordSource = "none"
	}

	if id == "" {
		return Host{}, hostValidationError("id", "不能为空")
	}
	if input.Name == "" {
		return Host{}, hostValidationError("name", "不能为空")
	}
	if input.Protocol == "" {
		input.Protocol = "ssh"
	}

	twoFactorEnabled := input.SSHTwoFactorEnabled != nil && *input.SSHTwoFactorEnabled
	otpEntryID := ""
	if input.SSHOTPEntryID != nil {
		otpEntryID = strings.TrimSpace(*input.SSHOTPEntryID)
	}
	otpAutoFill := input.SSHOTPAutoFill != nil && *input.SSHOTPAutoFill
	if len(otpEntryID) > maxSSHOTPEntryIDSize || !utf8.ValidString(otpEntryID) || strings.IndexByte(otpEntryID, 0) >= 0 {
		return Host{}, hostValidationError("sshOtpEntryId", "格式无效")
	}
	if input.Protocol != ProtocolSSH || !twoFactorEnabled {
		twoFactorEnabled = false
		otpEntryID = ""
		otpAutoFill = false
	} else if otpEntryID == "" {

		otpAutoFill = false
	}
	input.SSHTwoFactorEnabled = &twoFactorEnabled
	input.SSHOTPEntryID = &otpEntryID
	input.SSHOTPAutoFill = &otpAutoFill

	switch input.Protocol {
	case ProtocolSSH, ProtocolTelnet, ProtocolRDP, ProtocolVNC, ProtocolSerial:
	default:
		return Host{}, hostValidationError("protocol", "仅支持 SSH、Telnet、RDP、VNC 与串口")
	}
	if input.Address == "" {
		return Host{}, hostValidationError("address", "不能为空")
	}

	if input.Username == "" &&
		input.Protocol != ProtocolTelnet &&
		input.Protocol != ProtocolVNC &&
		input.Protocol != ProtocolSerial {
		return Host{}, hostValidationError("username", "不能为空")
	}
	if input.Port == 0 && input.Protocol == ProtocolSerial {

		input.Port = 0
	}
	if input.Port == 0 && input.Protocol != ProtocolSerial {

		switch input.Protocol {
		case ProtocolRDP:
			input.Port = defaultRDPPort
		case ProtocolVNC:
			input.Port = defaultVNCPort
		case ProtocolTelnet:
			input.Port = defaultTelnetPort
		default:
			input.Port = defaultSSHPort
		}
	}
	if input.Protocol != ProtocolSerial && (input.Port < 1 || input.Port > 65535) {
		return Host{}, hostValidationError("port", "必须在 1 到 65535 之间")
	}

	serialBaud, serialDataBits, serialParity, serialStopBits := 0, 0, "", ""
	if input.Protocol == ProtocolSerial {
		serialBaud = 115200
		if input.SerialBaud != nil && *input.SerialBaud != 0 {
			serialBaud = *input.SerialBaud
		}

		if serialBaud < 1 || serialBaud > 4294967295 {
			return Host{}, hostValidationError("serialBaud", "串口波特率必须在 1 到 4294967295 之间")
		}
		serialDataBits = 8
		if input.SerialDataBits != nil && *input.SerialDataBits != 0 {
			serialDataBits = *input.SerialDataBits
		}
		switch serialDataBits {
		case 5, 6, 7, 8:
		default:
			return Host{}, hostValidationError("serialDataBits", "数据位必须是 5、6、7 或 8")
		}
		serialParity = "none"
		if input.SerialParity != nil {
			if trimmed := strings.TrimSpace(*input.SerialParity); trimmed != "" {
				serialParity = trimmed
			}
		}
		switch serialParity {
		case "none", "odd", "even", "mark", "space":
		default:
			return Host{}, hostValidationError("serialParity", "校验位必须是 none、odd、even、mark 或 space")
		}
		serialStopBits = "1"
		if input.SerialStopBits != nil {
			if trimmed := strings.TrimSpace(*input.SerialStopBits); trimmed != "" {
				serialStopBits = trimmed
			}
		}
		switch serialStopBits {
		case "1", "1.5", "2":
		default:
			return Host{}, hostValidationError("serialStopBits", "停止位必须是 1、1.5 或 2")
		}
	}
	if len(input.CredentialID) > maxCredentialIDSize ||
		!utf8.ValidString(input.CredentialID) ||
		strings.IndexByte(input.CredentialID, 0) >= 0 {
		return Host{}, hostValidationError("credentialId", "格式无效")
	}
	if len(input.Notes) > maxHostNotesSize || !utf8.ValidString(input.Notes) || strings.IndexByte(input.Notes, 0) >= 0 {
		return Host{}, hostValidationError("notes", "格式无效")
	}
	now = now.UTC()
	return Host{
		ID:                  id,
		Icon:                input.Icon,
		Color:               input.Color,
		Name:                input.Name,
		Protocol:            input.Protocol,
		Address:             input.Address,
		Port:                input.Port,
		Username:            input.Username,
		Group:               input.Group,
		CredentialID:        input.CredentialID,
		PasswordSource:      input.PasswordSource,
		SSHTwoFactorEnabled: twoFactorEnabled,
		SSHOTPEntryID:       otpEntryID,
		SSHOTPAutoFill:      otpAutoFill,
		Notes:               input.Notes,
		SSHKeepAlive:        *input.SSHKeepAlive,
		LatencyDetection:    *input.LatencyDetection,
		AutoDetectOSIcon:    *input.AutoDetectOSIcon,
		DirectoryFollow:     input.DirectoryFollow,

		SFTPShowHiddenFiles:  *input.SFTPShowHiddenFiles,
		SFTPDefaultDirectory: sftpDefaultDirectoryValue(input.SFTPDefaultDirectory),

		RdpDisplayMode:      *input.RdpDisplayMode,
		RdpWidth:            *input.RdpWidth,
		RdpHeight:           *input.RdpHeight,
		RdpClipboardMode:    *input.RdpClipboardMode,
		RdpClipboardContent: *input.RdpClipboardContent,
		VncViewOnly:         *input.VncViewOnly,
		VncShared:           *input.VncShared,
		VncPixelDepth:       *input.VncPixelDepth,

		VncScale:                 *input.VncScale,
		VncAutoReconnect:         *input.VncAutoReconnect,
		VncAutoReconnectAttempts: *input.VncAutoReconnectAttempts,

		VncTransport:        *input.VncTransport,
		VncWsPath:           *input.VncWsPath,
		VncAllowInsecureTLS: *input.VncAllowInsecureTLS,

		VncRfbPasswordMode:           *input.VncRfbPasswordMode,
		Tags:                         input.Tags,
		ConnectionType:               *input.ConnectionType,
		TerminalType:                 *input.TerminalType,
		X11Forwarding:                *input.X11Forwarding,
		X11Trusted:                   *input.X11Trusted,
		BackspaceMode:                *input.BackspaceMode,
		Encoding:                     *input.Encoding,
		ConnectTimeoutSeconds:        *input.ConnectTimeoutSeconds,
		PostLoginCommand:             *input.PostLoginCommand,
		PostLoginCommandDelaySeconds: *input.PostLoginCommandDelaySeconds,
		EnvironmentVariables:         input.EnvironmentVariables,
		SerialBaud:                   serialBaud,
		SerialDataBits:               serialDataBits,
		SerialParity:                 serialParity,
		SerialStopBits:               serialStopBits,

		JumpHostID:              input.JumpHostID,
		ProxyID:                 input.ProxyID,
		ProxyMode:               input.ProxyMode,
		ProxyHost:               input.ProxyHost,
		ProxyPort:               input.ProxyPort,
		ProxyUsername:           input.ProxyUsername,
		ProxyPasswordConfigured: input.ProxyPassword != "",
		CreatedAt:               now,
		UpdatedAt:               now,
	}, nil
}

func NewGroup(id, name string, now time.Time) (Group, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		return Group{}, groupValidationError("id", "不能为空")
	}
	if name == "" {
		return Group{}, groupValidationError("name", "不能为空")
	}

	now = now.UTC()
	return Group{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

const (
	HostTagMaxCount  = 12
	HostTagMaxLength = 24
)

func NormalizeHostTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > HostTagMaxLength {
			return nil, hostValidationError("tags", fmt.Sprintf("单个标签最多 %d 个字符", HostTagMaxLength))
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	if len(normalized) > HostTagMaxCount {
		return nil, hostValidationError("tags", fmt.Sprintf("最多 %d 个标签", HostTagMaxCount))
	}
	return normalized, nil
}

func hostValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason, Err: ErrInvalidHost}
}

func groupValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason, Err: ErrInvalidGroup}
}

type Snippet struct {
	Icon         string    `json:"icon,omitempty"`
	Color        string    `json:"color,omitempty"`
	Pinned       bool      `json:"pinned,omitempty"`
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Content      string    `json:"content"`
	GroupID      string    `json:"groupId,omitempty"`
	Group        string    `json:"group,omitempty"`
	HostIDs      []string  `json:"hostIds,omitempty"`
	AssetGroupID string    `json:"assetGroupId,omitempty"`
	AssetGroup   string    `json:"assetGroup,omitempty"`
	AutoExecute  bool      `json:"autoExecute"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SnippetInput struct {
	Icon         string   `json:"icon,omitempty"`
	Color        string   `json:"color,omitempty"`
	Pinned       bool     `json:"pinned,omitempty"`
	Name         string   `json:"name"`
	Content      string   `json:"content"`
	GroupID      string   `json:"groupId,omitempty"`
	HostIDs      []string `json:"hostIds,omitempty"`
	AssetGroupID string   `json:"assetGroupId,omitempty"`
	AutoExecute  bool     `json:"autoExecute"`
}
