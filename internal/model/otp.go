package model

type OTPEntry struct {
	ID string `json:"id"`

	Name string `json:"name"`

	Username string `json:"username,omitempty"`

	HostID string `json:"hostId,omitempty"`

	Type string `json:"type"`

	Algorithm string `json:"algorithm"`
	Digits    int    `json:"digits"`

	Period int `json:"period"`

	Counter uint64 `json:"counter"`
}

type OTPCode struct {
	Code string `json:"code"`

	RemainingSeconds int `json:"remainingSeconds"`

	Counter uint64 `json:"counter"`
}

type OTPSaveInput struct {
	Entry  OTPEntry `json:"entry"`
	Secret string   `json:"secret"`
}
