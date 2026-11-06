package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SessionHistory struct {
	ID        string    `json:"id"`
	HostID    string    `json:"hostId"`
	HostName  string    `json:"hostName"`
	Address   string    `json:"address"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Status    string    `json:"status"`
	ErrorKey  string    `json:"errorKey"`
	Note      string    `json:"note"`
	Archived  bool      `json:"archived"`
}

func (h *SessionHistory) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID        string `json:"id"`
		HostID    string `json:"hostId"`
		HostName  string `json:"hostName"`
		Address   string `json:"address"`
		StartedAt string `json:"startedAt"`
		EndedAt   string `json:"endedAt"`
		Status    string `json:"status"`
		ErrorKey  string `json:"errorKey"`
		Note      string `json:"note"`
		Archived  bool   `json:"archived"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	startedAt, err := parseHistoryTime(raw.StartedAt)
	if err != nil {
		return fmt.Errorf("startedAt: %w", err)
	}
	endedAt, err := parseHistoryTime(raw.EndedAt)
	if err != nil {
		return fmt.Errorf("endedAt: %w", err)
	}
	*h = SessionHistory{
		ID:        raw.ID,
		HostID:    raw.HostID,
		HostName:  raw.HostName,
		Address:   raw.Address,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Status:    raw.Status,
		ErrorKey:  raw.ErrorKey,
		Note:      raw.Note,
		Archived:  raw.Archived,
	}
	return nil
}

func parseHistoryTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

type SessionHistoryClearResult struct {
	Sessions int `json:"sessions"`

	Recordings int `json:"recordings"`

	Skipped int `json:"skipped"`

	Failed int `json:"failed"`
}

type TerminalRecordingClearResult struct {
	Deleted int `json:"deleted"`

	Skipped int `json:"skipped"`

	Missing int `json:"missing"`

	Failed int `json:"failed"`
}

type SessionHistoryBatchArchiveResult struct {
	Updated int `json:"updated"`

	Missing int `json:"missing"`
}
