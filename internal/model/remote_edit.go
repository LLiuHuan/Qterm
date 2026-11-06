package model

import "time"

type LocalEditState string

const (
	LocalEditStateWatching LocalEditState = "watching"

	LocalEditStateSyncing LocalEditState = "syncing"

	LocalEditStateSynced LocalEditState = "synced"

	LocalEditStateError LocalEditState = "error"

	LocalEditStateStopped LocalEditState = "stopped"
)

type LocalEdit struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"sessionId"`
	RemotePath string         `json:"remotePath"`
	LocalPath  string         `json:"localPath"`
	State      LocalEditState `json:"state"`

	BaselineHash string `json:"baselineHash"`

	LastError string `json:"lastError"`

	HasUnsyncedChanges bool `json:"hasUnsyncedChanges"`
}

type RemoteFileContent struct {
	ContentBase64 string    `json:"contentBase64"`
	Size          int64     `json:"size"`
	Binary        bool      `json:"binary"`
	Truncated     bool      `json:"truncated"`
	Hash          string    `json:"hash"`
	ModTime       time.Time `json:"modTime"`
}

type RemoteFileWriteResult struct {
	Saved       bool   `json:"saved"`
	Conflict    bool   `json:"conflict"`
	CurrentHash string `json:"currentHash"`
}

type RemoteEditChanged struct {
	EditID    string         `json:"editID"`
	State     LocalEditState `json:"state"`
	LastError string         `json:"lastError"`

	HasUnsyncedChanges bool `json:"hasUnsyncedChanges"`
}
