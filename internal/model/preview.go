package model

import "time"

type RemoteImageContent struct {
	ContentBase64 string    `json:"contentBase64"`
	Size          int64     `json:"size"`
	MimeType      string    `json:"mimeType"`
	Truncated     bool      `json:"truncated"`
	ModTime       time.Time `json:"modTime"`
}
