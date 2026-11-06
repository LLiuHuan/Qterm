package store

import (
	"encoding/json"
	"strings"
	"time"
)

func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func encodeHostIDs(ids []string) string { b, _ := json.Marshal(ids); return string(b) }

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
