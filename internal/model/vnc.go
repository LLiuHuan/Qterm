package model

type VncSession struct {
	ID      string `json:"id"`
	HostID  string `json:"hostId"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`

	ViewOnly bool `json:"viewOnly"`

	Shared bool `json:"shared"`

	PixelDepth int `json:"pixelDepth"`

	Transport string `json:"transport"`

	AllowInsecureTLS bool `json:"allowInsecureTls"`

	ServerCertFingerprint string `json:"serverCertFingerprint,omitempty"`

	ServerSoftware string `json:"serverSoftware,omitempty"`
	ServerVersion  string `json:"serverVersion,omitempty"`

	ServerBanner string `json:"serverBanner,omitempty"`

	IdentitySource string `json:"identitySource,omitempty"`

	Connected bool `json:"connected"`

	Error string `json:"error,omitempty"`
}

type VncStreamTicket struct {
	StreamName string `json:"streamName"`
	Token      string `json:"token"`

	ProtocolVersion int `json:"protocolVersion"`
}

type VncOpenResult struct {
	Session VncSession      `json:"session"`
	Ticket  VncStreamTicket `json:"ticket"`

	Username string `json:"username,omitempty"`

	Password string `json:"password,omitempty"`

	Transport string `json:"transport"`

	AllowInsecureTLS bool `json:"allowInsecureTls"`

	BasicAuthUsed bool `json:"basicAuthUsed"`

	RfbPasswordMode string `json:"rfbPasswordMode"`

	ServerCertFingerprint string `json:"serverCertFingerprint,omitempty"`

	ServerSoftware string `json:"serverSoftware,omitempty"`
	ServerVersion  string `json:"serverVersion,omitempty"`
	ServerBanner   string `json:"serverBanner,omitempty"`
	IdentitySource string `json:"identitySource,omitempty"`
}

type VncHostProfile struct {
	HostID  string `json:"hostId"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`

	Username string `json:"username,omitempty"`

	PasswordAvailable bool `json:"passwordAvailable"`

	PasswordSource string `json:"passwordSource,omitempty"`

	ViewOnly   bool `json:"viewOnly"`
	Shared     bool `json:"shared"`
	PixelDepth int  `json:"pixelDepth"`

	Transport        string `json:"transport"`
	WsPath           string `json:"wsPath"`
	AllowInsecureTLS bool   `json:"allowInsecureTls"`

	BasicAuthUsed bool `json:"basicAuthUsed"`

	RfbPasswordMode string `json:"rfbPasswordMode"`

	RfbPasswordRequired bool `json:"rfbPasswordRequired"`
}
