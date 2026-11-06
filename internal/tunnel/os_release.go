package tunnel

import (
	"strings"
	"unicode"
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
	for _, marker := range []string{release.ID, release.IDLike, release.Name, release.PrettyName} {
		if icon := linuxLogoForMarker(marker); icon != "" {
			return icon
		}
	}
	return "linux"
}

func linuxLogoForMarker(marker string) string {
	tokens := osMarkerTokens(marker)
	marker = strings.ToLower(marker)
	switch {
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
