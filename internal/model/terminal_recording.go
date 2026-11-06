package model

import "time"

type TerminalRecording struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	HostID    string `json:"hostId"`

	Mode       string    `json:"mode"`
	Path       string    `json:"path"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	Status     string    `json:"status"`
	Bytes      int64     `json:"bytes"`
	EventCount int64     `json:"eventCount"`
	HostName   string    `json:"hostName"`
	Address    string    `json:"address"`
	Note       string    `json:"note"`
}

type TerminalRecordingPage struct {
	Items    []TerminalRecording `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int64               `json:"total"`
}

type TerminalRecordingEvent struct {
	AtMS    int64  `json:"atMs"`
	Type    uint8  `json:"type"`
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
	Data    string `json:"data"`
}

type TerminalRecordingEventBatch struct {
	Cursor    int64                    `json:"cursor"`
	Truncated bool                     `json:"truncated"`
	Events    []TerminalRecordingEvent `json:"events"`
}
