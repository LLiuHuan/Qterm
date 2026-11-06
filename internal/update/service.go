package update

import (
	"context"
	"errors"

	"changeme/internal/errno"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Options struct {
	Repository string

	CurrentVersion string

	APIBaseURL string

	Client *http.Client

	Emit func(Status)

	Now func() time.Time

	TempRoot string

	Executable func() (string, error)

	GOOS   string
	GOARCH string
}

type Service struct {
	repository     string
	currentVersion string
	apiBase        string
	apiClient      *http.Client
	downloadClient *http.Client
	emit           func(Status)
	now            func() time.Time
	tempRoot       string
	executable     func() (string, error)
	goos           string
	goarch         string

	mu sync.Mutex

	status Status

	release *Release

	checksumURL string
	stagedPath  string
	stagedDir   string
	cancel      context.CancelFunc
	busy        bool
	closed      bool
}

func NewService(options Options) *Service {
	client := options.Client
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()

		transport.Proxy = http.ProxyFromEnvironment
		client = &http.Client{Transport: transport}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	executable := options.Executable
	if executable == nil {
		executable = os.Executable
	}
	goos := options.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := options.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	apiBase := options.APIBaseURL
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	tempRoot := options.TempRoot
	if tempRoot == "" {
		tempRoot = os.TempDir()
	}
	service := &Service{
		repository:     strings.Trim(options.Repository, "/"),
		currentVersion: strings.TrimSpace(options.CurrentVersion),
		apiBase:        apiBase,

		apiClient:      &http.Client{Transport: client.Transport, Timeout: 30 * time.Second},
		downloadClient: &http.Client{Transport: client.Transport},
		emit:           options.Emit,
		now:            now,
		tempRoot:       tempRoot,
		executable:     executable,
		goos:           goos,
		goarch:         goarch,
	}
	service.status = Status{State: StateIdle, CurrentVersion: service.currentVersion}
	return service
}

func (s *Service) applyCurrentVersion(currentVersion string) {
	version := strings.TrimSpace(currentVersion)
	s.mu.Lock()
	if version == "" {
		version = s.currentVersion
	}
	if version == "" {
		version = "0.0.0"
	}
	s.currentVersion = version
	s.status.CurrentVersion = version
	s.mu.Unlock()
}

func (s *Service) Status() Status {
	if s == nil {
		return Status{State: StateIdle}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.clone()
}

func (s *Service) Check(ctx context.Context, currentVersion string) (Release, error) {
	if s == nil {
		return Release{}, checkFailed("service")
	}
	if s.repository == "" {
		return Release{}, checkFailed("repository")
	}
	s.applyCurrentVersion(currentVersion)
	s.setState(StateChecking, func(status *Status) {
		status.MessageKey = ""
		status.SafeDetail = ""
		status.Downloaded = 0
		status.Percent = 0
	})

	payload, err := s.fetchLatestRelease(ctx)
	if err != nil {

		if appErr := errno.Extract(err); appErr != nil && appErr.MessageKey == keyNoRelease {
			release := Release{NoRelease: true}
			s.mu.Lock()
			s.release = &release
			s.mu.Unlock()
			s.setState(StateNoRelease, func(status *Status) { status.Release = &release })
			return release, nil
		}
		s.fail(err)
		return Release{}, err
	}
	remoteVersion := strings.TrimPrefix(payload.TagName, "v")
	comparison, comparable := compareVersions(s.currentVersion, remoteVersion)
	if !comparable {
		err := checkFailed("version")
		s.fail(err)
		return Release{}, err
	}

	asset, hasAsset := pickAsset(payload, s.goos, s.goarch)
	checksum, hasChecksum := pickChecksumAsset(payload)
	release := Release{
		HasUpdate: comparison < 0,
		Version:   remoteVersion,
		Tag:       payload.TagName,
		Name:      payload.Name,
		Notes:     payload.Body,
		PageURL:   payload.HTMLURL,
		Published: payload.PublishedAt,
	}
	if hasAsset {
		release.Asset = Asset{Name: asset.Name, Size: asset.Size, URL: asset.BrowserDownloadURL}
	}

	s.mu.Lock()
	s.release = &release

	s.checksumURL = ""
	if hasChecksum {
		s.checksumURL = checksum.BrowserDownloadURL
	}
	s.mu.Unlock()

	switch {
	case !release.HasUpdate:
		s.setState(StateUpToDate, func(status *Status) { status.Release = &release })
		return release, nil
	case !hasAsset:

		err := noAsset(assetNameFor(s.goos, s.goarch))
		s.failWith(StateAvailable, err, &release)
		return release, err
	}
	s.setState(StateAvailable, func(status *Status) {
		status.Release = &release
		status.Total = asset.Size
		status.Downloaded = 0
		status.Percent = 0
	})
	return release, nil
}

func (s *Service) Download(ctx context.Context) error {
	if s == nil {
		return downloadFailed("service")
	}
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return downloadFailed("busy")
	}
	release := s.release
	stagedPath := s.stagedPath
	s.mu.Unlock()
	if release == nil || !release.HasUpdate {
		return downloadFailed("no-release")
	}
	if stagedPath != "" {

		s.setState(StateReady, nil)
		return nil
	}
	if release.Asset.URL == "" {
		return noAsset(assetNameFor(s.goos, s.goarch))
	}

	downloadCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.busy = true
	s.cancel = cancel
	s.mu.Unlock()
	s.setState(StateDownloading, func(status *Status) {
		status.Release = release
		status.Downloaded = 0
		status.Percent = 0
		status.Total = release.Asset.Size
	})

	path, err := s.downloadAsset(downloadCtx, release)
	cancel()
	s.mu.Lock()
	s.busy = false
	s.cancel = nil
	s.mu.Unlock()

	if err != nil {
		if errors.Is(err, context.Canceled) {

			s.setState(StateAvailable, func(status *Status) {
				status.Release = release
				status.Downloaded = 0
				status.Percent = 0
				status.MessageKey = keyCancelled
			})
			return nil
		}
		s.failWith(StateFailed, err, release)
		return err
	}

	s.mu.Lock()
	s.stagedPath = path
	s.stagedDir = filepath.Dir(path)
	s.mu.Unlock()
	s.setState(StateReady, func(status *Status) {
		status.Release = release
		status.Downloaded = release.Asset.Size
		status.Total = release.Asset.Size
		status.Percent = 100
	})
	return nil
}

func (s *Service) CheckAndDownload(ctx context.Context, currentVersion string) {
	release, err := s.Check(ctx, currentVersion)
	if err != nil || !release.HasUpdate {
		return
	}
	_ = s.Download(ctx)
}

func (s *Service) Install(ctx context.Context) error {
	if s == nil {
		return installFailed("service")
	}
	s.mu.Lock()
	stagedPath := s.stagedPath
	release := s.release
	s.mu.Unlock()
	if stagedPath == "" || release == nil {
		return installFailed("not-staged")
	}
	executable, err := s.executable()
	if err != nil {
		wrapped := installFailed("executable")
		s.failWith(StateFailed, wrapped, release)
		return wrapped
	}
	s.setState(StateInstalling, func(status *Status) { status.Release = release })
	if err := s.launchReplacer(ctx, stagedPath, executable); err != nil {
		s.failWith(StateFailed, err, release)
		return err
	}
	return nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	cancel := s.cancel
	dir := s.stagedDir
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	return nil
}

func (s *Service) setState(state State, mutate func(status *Status)) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.status.State = state
	s.status.CurrentVersion = s.currentVersion
	if s.release != nil && s.status.Release == nil {
		release := *s.release
		s.status.Release = &release
	}
	if mutate != nil {
		mutate(&s.status)
	}
	snapshot := s.status.clone()
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		emit(snapshot)
	}
}

func (s *Service) fail(err error) {
	s.failWith(StateFailed, err, nil)
}

func (s *Service) failWith(state State, err error, release *Release) {
	messageKey, detail := errorFields(err)
	s.setState(state, func(status *Status) {
		if release != nil {
			copied := *release
			status.Release = &copied
		}
		status.MessageKey = messageKey
		status.SafeDetail = detail
		status.Downloaded = 0
		status.Percent = 0
	})
}

func errorFields(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if extracted := errno.Extract(err); extracted != nil {
		return extracted.MessageKey, extracted.SafeDetail
	}
	return keyCheckFailed, ""
}
