package app

const settingsChangedEvent = "qterm:settings-changed"

type settingsChangedPayload struct {
	Key    string `json:"key"`
	Source uint   `json:"source"`
}

const settingsWindowStateEvent = "qterm:settings-window-state"

type settingsWindowStatePayload struct {
	Open bool `json:"open"`

	Search string `json:"search,omitempty"`

	Kind string `json:"kind,omitempty"`

	Category string `json:"category,omitempty"`
}
