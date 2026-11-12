package assetdata

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"changeme/internal/model"
)

type Snapshot struct {
	Version       int           `json:"version"`
	CreatedAt     time.Time     `json:"createdAt"`
	Hosts         []Host        `json:"hosts"`
	Groups        []model.Group `json:"groups"`
	Snippets      []Snippet     `json:"snippets"`
	SnippetGroups []model.Group `json:"snippetGroups"`
	Credentials   []Credential  `json:"credentials"`

	Proxies []Proxy `json:"proxies,omitempty"`

	CredentialPromptRules []model.CredentialPromptRule `json:"credentialPromptRules,omitempty"`
}

type Host struct {
	model.Host
	Password      []byte `json:"password,omitempty"`
	ProxyPassword []byte `json:"proxyPassword,omitempty"`
}

func (h *Host) UnmarshalJSON(data []byte) error {
	type plainHost Host
	defaultProxyMode := model.ProxyModeNone
	decoded := plainHost{Host: model.Host{
		SSHKeepAlive: true, LatencyDetection: true, AutoDetectOSIcon: true,
		DirectoryFollow: "inherit",

		SFTPShowHiddenFiles: "inherit",

		RdpClipboardMode:    "both",
		RdpClipboardContent: "text",
		VncPixelDepth:       32,

		VncShared:                true,
		VncScale:                 "fit",
		VncAutoReconnectAttempts: 5,

		VncTransport:        "tcp",
		VncWsPath:           "/",
		VncAllowInsecureTLS: false,

		BackspaceMode:         "del",
		Encoding:              "utf-8",
		ConnectTimeoutSeconds: 15,
		X11Trusted:            true,
		RdpDisplayMode:        "fit",
		RdpWidth:              1280,
		RdpHeight:             800,
		ConnectionType:        "standard",

		VncRfbPasswordMode: model.VncRfbPasswordAuto,

		ProxyMode: &defaultProxyMode,
	}}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*h = Host(decoded)
	return nil
}

func optionalHostString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

type Snippet struct {
	model.Snippet
	SortOrder int `json:"sortOrder"`
}

type Credential struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Kind      model.CredentialKind `json:"kind"`
	Username  string               `json:"username,omitempty"`
	PublicKey string               `json:"publicKey,omitempty"`
	Value     []byte               `json:"value"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type Proxy struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username,omitempty"`
	Password  []byte `json:"password,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Snapshot) Clear() {
	for i := range s.Hosts {
		clear(s.Hosts[i].Password)
		clear(s.Hosts[i].ProxyPassword)
	}
	for i := range s.Credentials {
		clear(s.Credentials[i].Value)
	}
	for i := range s.Proxies {
		clear(s.Proxies[i].Password)
	}
}

func (s *Snapshot) Validate() error {
	invalid := errors.New("invalid asset snapshot")
	if s.Version != 1 || len(s.Hosts)+len(s.Groups)+len(s.Snippets)+len(s.SnippetGroups)+len(s.Credentials)+len(s.CredentialPromptRules)+len(s.Proxies) > 100000 {
		return invalid
	}
	unique := func(ids []string) (map[string]bool, error) {
		out := make(map[string]bool, len(ids))
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || len(id) > 1024 || strings.ContainsRune(id, 0) || out[id] {
				return nil, invalid
			}
			out[id] = true
		}
		return out, nil
	}
	var ids []string
	for _, g := range s.Groups {
		ids = append(ids, g.ID)
		if strings.TrimSpace(g.Name) == "" {
			return invalid
		}
	}
	groups, err := unique(ids)
	if err != nil {
		return err
	}
	parents := make(map[string]string, len(s.Groups))
	for _, g := range s.Groups {
		if g.ParentID != "" && !groups[g.ParentID] {
			return invalid
		}
		parents[g.ID] = g.ParentID
	}

	done := map[string]bool{}
	for _, g := range s.Groups {
		seen := map[string]bool{}
		for id := g.ID; id != "" && !done[id]; id = parents[id] {
			if seen[id] {
				return invalid
			}
			seen[id] = true
		}
		for id := range seen {
			done[id] = true
		}
	}
	ids = nil
	for _, c := range s.Credentials {
		ids = append(ids, c.ID)
		if !c.Kind.Valid() || strings.TrimSpace(c.Name) == "" || len(c.Value) == 0 || len(c.Value) > 16<<20 {
			return invalid
		}
	}
	credentials, err := unique(ids)
	if err != nil {
		return err
	}
	ids = nil
	for _, p := range s.Proxies {
		ids = append(ids, p.ID)

		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Host) == "" ||
			p.Port < 1 || p.Port > 65535 ||
			!model.ValidProxyMode(p.Mode) || p.Mode == model.ProxyModeNone ||
			len(p.Username) > 1024 || len(p.Password) > 16<<20 {
			return invalid
		}
	}
	proxies, err := unique(ids)
	if err != nil {
		return err
	}
	ids = nil

	customIDs := make(map[string]bool, len(s.Hosts))
	for _, h := range s.Hosts {
		ids = append(ids, h.ID)
		keepAlive, latency, autoIcon := h.SSHKeepAlive, h.LatencyDetection, h.AutoDetectOSIcon
		showHidden, defaultDirectory := h.SFTPShowHiddenFiles, h.SFTPDefaultDirectory
		x11Forwarding, x11Trusted := h.X11Forwarding, h.X11Trusted

		hostInput := model.HostInput{
			Name: h.Name, Protocol: h.Protocol, Address: h.Address, Port: h.Port,
			Username: h.Username, CredentialID: h.CredentialID, Notes: h.Notes,

			Alias: h.Alias, CustomID: h.CustomID,
			Password:     string(h.Password),
			SSHKeepAlive: &keepAlive, LatencyDetection: &latency, AutoDetectOSIcon: &autoIcon,
			DirectoryFollow: h.DirectoryFollow,

			SFTPShowHiddenFiles:  &showHidden,
			SFTPDefaultDirectory: &defaultDirectory,
			BackspaceMode:        optionalHostString(h.BackspaceMode),
			Encoding:             optionalHostString(h.Encoding),
			PostLoginCommand:     optionalHostString(h.PostLoginCommand),
			ConnectionType:       optionalHostString(h.ConnectionType),
			TerminalType:         optionalHostString(h.TerminalType),

			VncRfbPasswordMode:   optionalHostString(h.VncRfbPasswordMode),
			Tags:                 h.Tags,
			EnvironmentVariables: h.EnvironmentVariables,
			X11Forwarding:        &x11Forwarding,
			X11Trusted:           &x11Trusted,

			ProxyMode:     h.ProxyMode,
			ProxyHost:     h.ProxyHost,
			ProxyPort:     h.ProxyPort,
			ProxyUsername: h.ProxyUsername,
		}
		if h.ConnectTimeoutSeconds != 0 {
			value := h.ConnectTimeoutSeconds
			hostInput.ConnectTimeoutSeconds = &value
		}
		if h.PostLoginCommandDelaySeconds != 0 {
			value := h.PostLoginCommandDelaySeconds
			hostInput.PostLoginCommandDelaySeconds = &value
		}
		if h.RdpDisplayMode != "" {
			value := h.RdpDisplayMode
			hostInput.RdpDisplayMode = &value
		}
		if h.RdpWidth > 0 {
			value := h.RdpWidth
			hostInput.RdpWidth = &value
		}
		if h.RdpHeight > 0 {
			value := h.RdpHeight
			hostInput.RdpHeight = &value
		}
		if h.RdpClipboardMode != "" {
			value := h.RdpClipboardMode
			hostInput.RdpClipboardMode = &value
		}
		if h.RdpClipboardContent != "" {
			value := h.RdpClipboardContent
			hostInput.RdpClipboardContent = &value
		}
		if h.VncViewOnly {
			value := h.VncViewOnly
			hostInput.VncViewOnly = &value
		}
		if h.VncShared {
			value := h.VncShared
			hostInput.VncShared = &value
		}
		if h.VncPixelDepth > 0 {
			value := h.VncPixelDepth
			hostInput.VncPixelDepth = &value
		}

		if h.VncScale != "" {
			value := h.VncScale
			hostInput.VncScale = &value
		}
		if h.VncAutoReconnect {
			value := h.VncAutoReconnect
			hostInput.VncAutoReconnect = &value
		}
		if h.VncAutoReconnectAttempts > 0 {
			value := h.VncAutoReconnectAttempts
			hostInput.VncAutoReconnectAttempts = &value
		}

		if h.VncTransport != "" {
			value := h.VncTransport
			hostInput.VncTransport = &value
		}

		if h.VncWsPath != "" {
			value := h.VncWsPath
			hostInput.VncWsPath = &value
		}
		if h.VncAllowInsecureTLS {
			value := h.VncAllowInsecureTLS
			hostInput.VncAllowInsecureTLS = &value
		}
		hostInput.ProxyPassword = string(h.ProxyPassword)
		if _, err := model.NewHost(h.ID, hostInput, time.Now()); err != nil {
			return invalid
		}

		if customID := strings.ToLower(strings.TrimSpace(h.CustomID)); customID != "" {
			if customIDs[customID] {
				return invalid
			}
			customIDs[customID] = true
		}
		if h.GroupID != "" && !groups[h.GroupID] || h.CredentialID != "" && !credentials[h.CredentialID] || len(h.Password) > 16<<20 || len(h.ProxyPassword) > 16<<20 {
			return invalid
		}
	}
	hosts, err := unique(ids)
	if err != nil {
		return err
	}

	jumpParents := make(map[string]string, len(s.Hosts))
	for _, h := range s.Hosts {
		if h.JumpHostID != nil && *h.JumpHostID != "" {
			if !hosts[*h.JumpHostID] {
				return invalid
			}
			jumpParents[h.ID] = *h.JumpHostID
		}
		if h.ProxyID != "" && !proxies[h.ProxyID] {
			return invalid
		}
	}

	doneJump := map[string]bool{}
	for _, h := range s.Hosts {
		seen := map[string]bool{}
		for id := h.ID; id != "" && !doneJump[id]; id = jumpParents[id] {
			if seen[id] {
				return invalid
			}
			seen[id] = true
		}
		for id := range seen {
			doneJump[id] = true
		}
	}
	ids = nil
	for _, g := range s.SnippetGroups {
		ids = append(ids, g.ID)
		if strings.TrimSpace(g.Name) == "" {
			return invalid
		}
	}
	snippetGroups, err := unique(ids)
	if err != nil {
		return err
	}
	ids = nil
	for _, v := range s.Snippets {
		ids = append(ids, v.ID)
		if strings.TrimSpace(v.Name) == "" || v.GroupID != "" && !snippetGroups[v.GroupID] || v.AssetGroupID != "" && !groups[v.AssetGroupID] {
			return invalid
		}
		for _, id := range v.HostIDs {
			if !hosts[id] {
				return invalid
			}
		}
	}
	_, err = unique(ids)
	if err != nil {
		return err
	}

	ids = nil
	for _, rule := range s.CredentialPromptRules {
		ids = append(ids, rule.ID)
		enabled := rule.Enabled
		if _, err := model.NewCredentialPromptRule(rule.ID, model.CredentialPromptRuleInput{
			Kind:    rule.Kind,
			Pattern: rule.Pattern,
			Enabled: &enabled,
		}, rule.CreatedAt); err != nil {
			return invalid
		}
	}
	_, err = unique(ids)
	return err
}
