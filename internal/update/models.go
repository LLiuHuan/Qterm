package update

import "time"

type State string

const (
	StateIdle     State = "idle"
	StateChecking State = "checking"
	StateUpToDate State = "up-to-date"

	StateNoRelease   State = "no-release"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateInstalling  State = "installing"
	StateFailed      State = "failed"
)

type Asset struct {
	Name string `json:"name"`

	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type Release struct {
	HasUpdate bool `json:"hasUpdate"`

	NoRelease bool `json:"noRelease"`

	Version   string    `json:"version"`
	Tag       string    `json:"tag"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	PageURL   string    `json:"pageUrl"`
	Published time.Time `json:"publishedAt"`
	Asset     Asset     `json:"asset"`
}

type Status struct {
	State State `json:"state"`

	CurrentVersion string `json:"currentVersion"`

	Release *Release `json:"release,omitempty"`

	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
	Percent    int   `json:"percent"`

	MessageKey string `json:"messageKey,omitempty"`

	SafeDetail string `json:"safeDetail,omitempty"`
}

func (s Status) clone() Status {
	out := s
	if s.Release != nil {
		release := *s.Release
		out.Release = &release
	}
	return out
}
