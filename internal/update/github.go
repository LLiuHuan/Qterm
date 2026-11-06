package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBase = "https://api.github.com"

	checksumAssetName = "SHA256SUMS"

	maxMetadataBytes = 2 << 20

	maxNotesBytes = 64 << 10
)

type releaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releasePayload struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	Assets      []releaseAsset `json:"assets"`
}

func (s *Service) fetchLatestRelease(ctx context.Context) (*releasePayload, error) {
	endpoint := strings.TrimSuffix(s.apiBase, "/") + "/repos/" + s.repository + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, checkFailed("request")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Qterm-Updater")

	response, err := s.apiClient.Do(request)
	if err != nil {

		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, checkFailed("cancelled")
		}
		return nil, checkFailed("network")
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, noRelease()
	case http.StatusForbidden, http.StatusTooManyRequests:

		if reset := rateLimitReset(response); reset != "" {
			return nil, rateLimited(reset)
		}
		return nil, rateLimited("HTTP " + strconv.Itoa(response.StatusCode))
	default:
		return nil, checkFailed("HTTP " + strconv.Itoa(response.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataBytes))
	if err != nil {
		return nil, checkFailed("read")
	}
	var payload releasePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, checkFailed("decode")
	}
	if payload.TagName == "" {
		return nil, noRelease()
	}
	if len(payload.Body) > maxNotesBytes {
		payload.Body = payload.Body[:maxNotesBytes]
	}
	return &payload, nil
}

func rateLimitReset(response *http.Response) string {
	if response.Header.Get("X-RateLimit-Remaining") != "0" {
		return ""
	}
	raw := response.Header.Get("X-RateLimit-Reset")
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func assetNameFor(goos, goarch string) string {
	extension := "zip"
	if goos == "linux" {

		extension = "tar.gz"
	}
	return "qterm-" + goos + "-" + goarch + "." + extension
}

func archAliases(goarch string) []string {
	switch goarch {
	case "amd64":
		return []string{"amd64", "x86_64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{goarch}
	}
}

func pickAsset(payload *releasePayload, goos, goarch string) (releaseAsset, bool) {
	for _, arch := range archAliases(goarch) {
		wanted := assetNameFor(goos, arch)
		for _, asset := range payload.Assets {
			if asset.Name == wanted && asset.BrowserDownloadURL != "" {
				return asset, true
			}
		}
	}
	return releaseAsset{}, false
}

func pickChecksumAsset(payload *releasePayload) (releaseAsset, bool) {
	for _, asset := range payload.Assets {
		if asset.Name == checksumAssetName && asset.BrowserDownloadURL != "" {
			return asset, true
		}
	}
	return releaseAsset{}, false
}
