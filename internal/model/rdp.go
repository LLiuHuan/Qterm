package model

type RdpOpenInput struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`

	Engine string `json:"engine,omitempty"`
}

type RdpHelperEngine struct {
	Engine  string `json:"engine"`
	Ready   bool   `json:"ready"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type RdpFrame struct {
	SessionID     string `json:"sessionId"`
	Sequence      uint64 `json:"sequence"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Changed       bool   `json:"changed"`
	PNG           []byte `json:"png"`
	CursorVisible bool   `json:"cursorVisible"`
	CursorX       int    `json:"cursorX"`
	CursorY       int    `json:"cursorY"`
	CursorWidth   int    `json:"cursorWidth"`
	CursorHeight  int    `json:"cursorHeight"`
	CursorPNG     []byte `json:"cursorPng"`
}

type RdpStreamTicket struct {
	StreamName string `json:"streamName"`
	Token      string `json:"token"`

	ProtocolVersion int `json:"protocolVersion"`
}

type RdpHostProfile struct {
	HostID   string `json:"hostId"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Username string `json:"username"`

	Domain string `json:"domain,omitempty"`

	PasswordAvailable bool `json:"passwordAvailable"`

	PasswordSource string `json:"passwordSource,omitempty"`

	DisplayMode string `json:"displayMode,omitempty"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	ClipboardMode string `json:"clipboardMode,omitempty"`

	ClipboardContent string `json:"clipboardContent,omitempty"`
}

type RdpSession struct {
	ID            string            `json:"id"`
	Host          string            `json:"host"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Connected     bool              `json:"connected"`
	Error         string            `json:"error,omitempty"`
	SecurityLabel string            `json:"securityLabel"`
	Channels      map[string]uint16 `json:"channels,omitempty"`

	Engine string `json:"engine,omitempty"`
}

type RdpStats struct {
	SessionID      string `json:"sessionId"`
	Connected      bool   `json:"connected"`
	Closed         bool   `json:"closed"`
	Error          string `json:"error,omitempty"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Packets        int    `json:"packets"`
	ChannelPackets int    `json:"channelPackets"`
	BitmapUpdates  int    `json:"bitmapUpdates"`
	OrderUpdates   int    `json:"orderUpdates"`
	PointerUpdates int    `json:"pointerUpdates"`
	OrdersApplied  int    `json:"ordersApplied"`
	ClipboardText  string `json:"clipboardText,omitempty"`

	ClipboardSequence uint64 `json:"clipboardSequence,omitempty"`

	Engine       string `json:"engine,omitempty"`
	HelperStderr string `json:"helperStderr,omitempty"`

	ResizeUnsupported bool `json:"resizeUnsupported,omitempty"`

	CertificateState string `json:"certificateState"`

	CertificateFingerprint string `json:"certificateFingerprint,omitempty"`

	CertificateSubject string `json:"certificateSubject,omitempty"`
	CertificateIssuer  string `json:"certificateIssuer,omitempty"`

	ServerFPS float64 `json:"serverFPS,omitempty"`
}

type RdpTrustedCertificate struct {
	HostID  string `json:"hostId"`
	Address string `json:"address"`
	Port    int    `json:"port"`

	Fingerprint string `json:"fingerprint,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Issuer      string `json:"issuer,omitempty"`

	TrustedAt string `json:"trustedAt,omitempty"`
}
