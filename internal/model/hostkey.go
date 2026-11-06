package model

type HostKeyIssue string

const (
	HostKeyIssueUnknown HostKeyIssue = "unknown"
	HostKeyIssueChanged HostKeyIssue = "changed"
)

type HostKeyChallenge struct {
	Address string `json:"address"`

	HopIndex int `json:"hopIndex"`

	HopLabel            string       `json:"hopLabel"`
	Algorithm           string       `json:"algorithm"`
	SHA256Fingerprint   string       `json:"sha256Fingerprint"`
	Issue               HostKeyIssue `json:"issue"`
	TrustedFingerprints []string     `json:"trustedFingerprints"`
}

type HostKeyPending struct {
	SessionID  string             `json:"sessionId"`
	Challenge  HostKeyChallenge   `json:"challenge"`
	Challenges []HostKeyChallenge `json:"challenges"`
}

type KnownHostEntry struct {
	Host string `json:"host"`

	KeyType string `json:"keyType"`

	SHA256Fingerprint string `json:"sha256Fingerprint"`

	Comment string `json:"comment"`

	Marker string `json:"marker"`

	Source string `json:"source"`

	Line int `json:"line"`

	Editable bool `json:"editable"`
}

const (
	KnownHostSourceManaged = "managed"
	KnownHostSourceSystem  = "system"
)
