package model

type ForwardKind string

const (
	ForwardLocal   ForwardKind = "local"
	ForwardRemote  ForwardKind = "remote"
	ForwardDynamic ForwardKind = "dynamic"
)

type ForwardConfig struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Kind ForwardKind `json:"kind"`

	HostID     string `json:"hostId"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`

	Proxy string `json:"proxy,omitempty"`

	AutoOpen bool `json:"autoOpen"`
}

type ForwardState string

const (
	ForwardStopped      ForwardState = "stopped"
	ForwardStarting     ForwardState = "starting"
	ForwardRunning      ForwardState = "running"
	ForwardReconnecting ForwardState = "reconnecting"
	ForwardDisconnected ForwardState = "disconnected"
	ForwardError        ForwardState = "error"
)

type ForwardStatus struct {
	ID string `json:"id"`

	State ForwardState `json:"state"`

	Error string `json:"error,omitempty"`

	ListenPort int `json:"listenPort"`

	RetryAttempt int `json:"retryAttempt"`

	UpdatedAt int64 `json:"updatedAt"`
}
