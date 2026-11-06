package model

type ServiceInfo struct {
	Name string `json:"name"`

	Description string `json:"description"`

	LoadState   string `json:"loadState"`
	ActiveState string `json:"activeState"`
	SubState    string `json:"subState"`

	EnabledState string `json:"enabledState"`

	Custom bool `json:"custom"`
}

type ServiceSnapshot struct {
	Manager   string        `json:"manager"`
	Services  []ServiceInfo `json:"services"`
	UpdatedAt int64         `json:"updatedAt"`
}

type ServiceDetail struct {
	Name   string            `json:"name"`
	Fields map[string]string `json:"fields"`
}

type ServiceLog struct {
	Name string `json:"name"`
	Text string `json:"text"`

	Truncated bool `json:"truncated"`
}
