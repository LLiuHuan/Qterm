package model

type RouteTestHop struct {
	Kind string `json:"kind"`

	Label string `json:"label"`

	Address string `json:"address"`

	Reachable bool `json:"reachable"`

	Stage string `json:"stage"`

	ErrorKey string `json:"errorKey"`

	LatencyMs int64 `json:"latencyMs"`

	UnverifiedHostKey bool `json:"unverifiedHostKey"`
}

type RouteTestResult struct {
	Mode string         `json:"mode"`
	Hops []RouteTestHop `json:"hops"`
	OK   bool           `json:"ok"`
}

type RouteDraftInput struct {
	HostID     string `json:"hostId,omitempty"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	JumpHostID string `json:"jumpHostId,omitempty"`

	ProxyID            string `json:"proxyId,omitempty"`
	ProxyMode          string `json:"proxyMode,omitempty"`
	ProxyHost          string `json:"proxyHost,omitempty"`
	ProxyPort          int    `json:"proxyPort,omitempty"`
	ProxyUsername      string `json:"proxyUsername,omitempty"`
	ProxyPassword      string `json:"proxyPassword,omitempty"`
	ClearProxyPassword bool   `json:"clearProxyPassword,omitempty"`
	CredentialID       string `json:"credentialId,omitempty"`

	Protocol string `json:"protocol,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Secret   string `json:"secret,omitempty"`
}
