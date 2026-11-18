package model

type LaunchIntent struct {
	Kind string `json:"kind"`

	Target string `json:"target"`
}

const (
	LaunchIntentURL  = "url"
	LaunchIntentOpen = "open"
)
