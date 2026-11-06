package model

import "time"

type CommandHistory struct {
	ID         string    `json:"id"`
	HostID     string    `json:"hostId"`
	HostName   string    `json:"hostName"`
	SessionID  string    `json:"sessionId"`
	Command    string    `json:"command"`
	ExecutedAt time.Time `json:"executedAt"`
	Suppressed bool      `json:"suppressed"`
}
