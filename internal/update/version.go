package update

import (
	"strconv"
	"strings"
)

func parseVersion(raw string) (segments []int, prerelease string, ok bool) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if value == "" {
		return nil, "", false
	}

	if index := strings.IndexByte(value, '+'); index >= 0 {
		value = value[:index]
	}
	if index := strings.IndexByte(value, '-'); index >= 0 {
		prerelease = value[index+1:]
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nil, "", false
	}
	for _, part := range parts {
		if part == "" {
			return nil, "", false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return nil, "", false
		}
		segments = append(segments, number)
	}
	return segments, prerelease, true
}

func compareVersions(current, latest string) (result int, ok bool) {
	currentSegments, currentPre, currentOK := parseVersion(current)
	latestSegments, latestPre, latestOK := parseVersion(latest)
	if !currentOK || !latestOK {
		return 0, false
	}
	for index := 0; index < 3; index++ {
		left := segmentAt(currentSegments, index)
		right := segmentAt(latestSegments, index)
		if left == right {
			continue
		}
		if left < right {
			return -1, true
		}
		return 1, true
	}
	switch {
	case currentPre == latestPre:
		return 0, true
	case currentPre == "":
		return 1, true
	case latestPre == "":
		return -1, true
	case currentPre < latestPre:
		return -1, true
	default:
		return 1, true
	}
}

func segmentAt(segments []int, index int) int {
	if index < len(segments) {
		return segments[index]
	}
	return 0
}
