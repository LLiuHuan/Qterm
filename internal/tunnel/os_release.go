package tunnel

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	remoteOSProbeUnixCommand = "uname -s"

	remoteOSProbeWindowsCIM = `powershell -NoProfile -Command "(Get-CimInstance Win32_OperatingSystem).Caption; (Get-CimInstance Win32_OperatingSystem).Version"`
	remoteOSProbeWindowsVer = "cmd /c ver"
	remoteOSProbeMacOSVer   = "sw_vers -productVersion"

	osReleaseProbeCommand = "cat /etc/os-release 2>/dev/null || cat /usr/lib/os-release 2>/dev/null || cat /system/etc/os-release 2>/dev/null"
)

type osReleaseInfo struct {
	PrettyName string
	ID         string
	Name       string
	IDLike     string
}

func parseOSRelease(output string) osReleaseInfo {
	var result osReleaseInfo
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = cleanOSReleaseValue(value)
		switch key {
		case "PRETTY_NAME":
			result.PrettyName = value
		case "ID":
			result.ID = value
		case "NAME":
			result.Name = value
		case "ID_LIKE":
			result.IDLike = value
		}
	}
	return result
}

func cleanOSReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func linuxDistributionIcon(output string) string {
	release := parseOSRelease(output)
	upstream := ""
	for _, marker := range []string{release.ID, release.IDLike} {
		if icon := linuxLogoForMarker(marker); icon != "" {
			if !isUpstreamDistroIcon(icon) {
				return icon
			}
			if upstream == "" {
				upstream = icon
			}
		}
	}
	for _, marker := range []string{release.Name, release.PrettyName} {
		if icon := linuxLogoForMarker(marker); icon != "" && !isUpstreamDistroIcon(icon) {
			return icon
		}
	}
	if upstream != "" {
		return upstream
	}
	if icon := linuxLogoForMarker(release.Name); icon != "" {
		return icon
	}
	if icon := linuxLogoForMarker(release.PrettyName); icon != "" {
		return icon
	}
	return "linux"
}

func isUpstreamDistroIcon(icon string) bool {
	switch icon {
	case "debian", "ubuntu", "centos", "rhel", "red-hat", "linux":
		return true
	default:
		return false
	}
}

func linuxLogoForMarker(marker string) string {
	tokens := osMarkerTokens(marker)
	marker = strings.ToLower(marker)
	switch {
	case hasAnyOSMarker(tokens, "armbian"):
		return "armbian"
	case hasAnyOSMarker(tokens, "almalinux", "alma"):
		return "alma-linux"
	case hasAnyOSMarker(tokens, "alpine"):
		return "alpine-linux"
	case hasAnyOSMarker(tokens, "amzn", "amazon", "amazonlinux"):
		return "amazon-linux"
	case hasAnyOSMarker(tokens, "arch", "archlinux"):
		return "arch-linux"
	case hasAnyOSMarker(tokens, "debian"):
		return "debian"
	case hasAnyOSMarker(tokens, "fedora"):
		return "fedora"
	case hasAnyOSMarker(tokens, "gentoo"):
		return "gentoo"
	case hasAnyOSMarker(tokens, "kali"):
		return "kali-linux"
	case hasAnyOSMarker(tokens, "manjaro"):
		return "manjaro"
	case hasAnyOSMarker(tokens, "nixos", "nix"):
		return "nixos"
	case hasAnyOSMarker(tokens, "opensuse", "suse", "sles", "sled"):
		return "opensuse"
	case hasAnyOSMarker(tokens, "openwrt"):
		return "openwrt"
	case hasAnyOSMarker(tokens, "oracle", "oraclelinux", "ol"):
		return "oracle-linux"
	case hasAnyOSMarker(tokens, "popos", "pop"):
		return "pop-os"
	case hasAnyOSMarker(tokens, "raspbian", "raspios", "raspberry", "raspberrypi"):
		return "raspberry-pi"
	case hasAnyOSMarker(tokens, "rhel"):
		return "rhel"
	case hasAnyOSMarker(tokens, "redhat") || hasAllOSMarkers(tokens, "red", "hat"):
		return "red-hat"
	case hasAnyOSMarker(tokens, "steamos", "steam"):
		return "steamos"
	case hasAnyOSMarker(tokens, "elementary"):
		return "elementary"
	case hasAnyOSMarker(tokens, "deepin"):
		return "deepin"
	case hasAnyOSMarker(tokens, "parrot"):
		return "parrot-security"
	case hasAnyOSMarker(tokens, "void"):
		return "void-linux"
	case hasAnyOSMarker(tokens, "uos", "uniontech", "uniontechos") || hasAnyOSSubstring(marker, "统信"):
		return "uos"
	case hasAnyOSMarker(tokens, "openkylin") || hasAnyOSSubstring(marker, "开放麒麟"):
		return "openkylin"
	case hasAnyOSMarker(tokens, "neokylin") || hasAnyOSSubstring(marker, "中标麒麟"):
		return "neokylin"
	case hasAnyOSMarker(tokens, "kylin") || hasAnyOSSubstring(marker, "银河麒麟", "麒麟"):
		return "kylin"
	case hasAnyOSMarker(tokens, "openeuler", "open-euler"):
		return "openeuler"
	case hasAnyOSMarker(tokens, "euleros", "euler"):
		return "euleros"
	case hasAnyOSMarker(tokens, "anolis", "anolisos") || hasAnyOSSubstring(marker, "龙蜥"):
		return "anolis"
	case hasAnyOSMarker(tokens, "loongnix") || hasAnyOSSubstring(marker, "龙芯"):
		return "loongnix"
	case hasAnyOSMarker(tokens, "tencentos", "tencent"):
		return "tencentos"
	case hasAnyOSMarker(tokens, "alinux", "alibabacloud", "alibaba"):
		return "alinux"
	case hasAnyOSMarker(tokens, "fnos", "feiniu") || hasAnyOSSubstring(marker, "飞牛"):
		return "fnos"
	case hasAnyOSMarker(tokens, "opencloudos"):
		return "opencloudos"
	case hasAnyOSMarker(tokens, "chromeos", "chromiumos", "cros"):
		return "chromeos"
	case hasAnyOSMarker(tokens, "harmonyos", "openharmony"):
		return "harmonyos"
	case hasAnyOSMarker(tokens, "rocky", "rockylinux"):
		return "rocky-linux"
	case hasAnyOSMarker(tokens, "centos"):
		return "centos"
	case hasAnyOSMarker(tokens, "ubuntu"):
		return "ubuntu"
	case hasAnyOSMarker(tokens, "linuxmint", "mint"):
		return "linux-mint"
	case hasAnyOSMarker(tokens, "android"):
		return "android"
	default:
		return ""
	}
}

func hasAnyOSSubstring(value string, markers ...string) bool {
	for _, marker := range markers {
		if strings.Contains(value, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func hasAllOSMarkers(tokens map[string]struct{}, values ...string) bool {
	for _, value := range values {
		if _, ok := tokens[value]; !ok {
			return false
		}
	}
	return true
}

func osMarkerTokens(value string) map[string]struct{} {
	tokens := make(map[string]struct{})
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		tokens[strings.ToLower(token.String())] = struct{}{}
		token.Reset()
	}
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			token.WriteRune(char)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func hasAnyOSMarker(tokens map[string]struct{}, values ...string) bool {
	for _, value := range values {
		if _, ok := tokens[value]; ok {
			return true
		}
	}
	return false
}

func remoteOSIcon(output string) (string, bool) {
	marker := firstNonEmptyProbeLine(output)
	switch {
	case marker == "darwin" || strings.HasPrefix(marker, "darwin "):
		return "macos", true
	case marker == "linux" || strings.HasPrefix(marker, "linux "):
		return "linux", true
	case strings.HasPrefix(marker, "mingw") || strings.HasPrefix(marker, "msys"),
		strings.HasPrefix(marker, "cygwin"),
		strings.HasPrefix(marker, "microsoft windows"), strings.HasPrefix(marker, "windows"):

		return "windows", true
	case marker == "freebsd" || strings.HasPrefix(marker, "freebsd "):
		return "freebsd", true
	case marker == "openbsd" || strings.HasPrefix(marker, "openbsd "):
		return "openbsd", true
	case marker == "netbsd" || strings.HasPrefix(marker, "netbsd "):
		return "netbsd", true
	case marker == "dragonfly" || strings.HasPrefix(marker, "dragonfly "):
		return "dragonfly-bsd", true
	case marker == "illumos" || strings.HasPrefix(marker, "illumos "):
		return "illumos", true
	case marker == "aix" || strings.HasPrefix(marker, "aix "):
		return "aix", true
	case marker == "sunos" || strings.HasPrefix(marker, "sunos "),
		marker == "hp-ux" || strings.HasPrefix(marker, "hp-ux "),
		marker == "unix" || strings.HasPrefix(marker, "unix "):
		return "unix", true
	default:
		return "", false
	}
}

func firstNonEmptyProbeLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.ToLower(strings.TrimSpace(line)); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func probeLines(output string) []string {
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func windowsIconForOutput(output string) string {
	lines := probeLines(output)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "windows") {
			continue
		}
		switch {
		case strings.Contains(lower, "windows 11"):
			return "windows-11"
		case strings.Contains(lower, "windows 7"):
			return "windows-7"
		case strings.Contains(lower, "windows 10"):
			return "windows-10"
		}
	}
	for _, line := range lines {
		build, ok := windowsBuildNumber(line)
		if !ok {
			continue
		}
		switch {
		case build >= 22000:
			return "windows-11"
		case build >= 10240:

			return "windows-10"
		case build >= 7600 && build < 8000:
			return "windows-7"
		}
	}
	return ""
}

func windowsBuildNumber(line string) (int, bool) {
	for index := 0; index < len(line); index++ {
		if !isASCIIDigit(line[index]) {
			continue
		}
		start := index
		for index < len(line) && isASCIIDigit(line[index]) {
			index++
		}

		if start > 0 && isASCIIDigit(line[start-1]) {
			continue
		}
		major, err := strconv.Atoi(line[start:index])
		if err != nil || (major != 6 && major != 10) {
			continue
		}
		buildText, ok := leadingVersionTail(line[index:])
		if !ok {
			continue
		}
		if build, convErr := strconv.Atoi(buildText); convErr == nil {
			return build, true
		}
	}
	return 0, false
}

func leadingVersionTail(rest string) (string, bool) {
	rest = strings.TrimPrefix(rest, ".")
	minorEnd := 0
	for minorEnd < len(rest) && isASCIIDigit(rest[minorEnd]) {
		minorEnd++
	}
	if minorEnd == 0 || minorEnd >= len(rest) || rest[minorEnd] != '.' {
		return "", false
	}
	tail := rest[minorEnd+1:]
	buildEnd := 0
	for buildEnd < len(tail) && isASCIIDigit(tail[buildEnd]) {
		buildEnd++
	}
	if buildEnd == 0 {
		return "", false
	}
	return tail[:buildEnd], true
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

func macosVersionIcon(output string) string {
	for _, line := range probeLines(output) {
		candidate := strings.ToLower(line)
		if _, rest, ok := strings.Cut(candidate, ":"); ok {
			candidate = rest
		}
		candidate = strings.TrimSpace(candidate)
		if dot := strings.Index(candidate, "."); dot > 0 {
			candidate = candidate[:dot]
		}
		major, err := strconv.Atoi(candidate)
		if err != nil || major <= 0 || major > 99 {
			continue
		}
		return "macos-" + strconv.Itoa(major)
	}
	return ""
}
