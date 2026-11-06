package x11

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("no local X server available")

type Settings struct {
	Display string `json:"display"`

	XauthPath string `json:"xauthPath"`

	AuthorityFile string `json:"authorityFile"`
}

type Result struct {
	Available bool `json:"available"`

	Display string `json:"display"`

	XauthPath string `json:"xauthPath"`

	AuthorityFile string `json:"authorityFile"`

	Cookie string `json:"-"`

	AuthProtocol string `json:"authProtocol"`

	Screen int `json:"screen"`

	Reason string `json:"reason"`

	Notice string `json:"notice"`

	Detail string `json:"detail"`
}

const (
	ReasonOK              = "x11.ok"
	ReasonNoDisplay       = "x11.no_display"
	ReasonXauthMissing    = "x11.xauth_missing"
	ReasonNoCookie        = "x11.no_cookie"
	ReasonServerUnreach   = "x11.server_unreachable"
	ReasonUnsupportedHost = "x11.unsupported_platform"

	NoticeFakeCookie = "x11.fake_cookie"
)

const detectTimeout = 5 * time.Second

func Detect(ctx context.Context, settings Settings) (Result, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), detectTimeout)
		defer cancel()
	}
	platform := runtime.GOOS
	switch platform {
	case "darwin", "linux", "windows":
		return detectForPlatform(ctx, platform, settings)
	default:
		return Result{Reason: ReasonUnsupportedHost, Detail: platform}, ErrUnavailable
	}
}

func detectForPlatform(ctx context.Context, platform string, settings Settings) (Result, error) {
	result := Result{AuthProtocol: "MIT-MAGIC-COOKIE-1"}
	result.Display = strings.TrimSpace(settings.Display)
	if result.Display == "" {
		result.Display = defaultDisplay(platform)
	}
	if result.Display == "" {
		result.Reason = ReasonNoDisplay
		return result, ErrUnavailable
	}
	result.Screen = screenNumber(result.Display)

	result.XauthPath = strings.TrimSpace(settings.XauthPath)
	if result.XauthPath == "" {
		result.XauthPath = defaultXauthPath(platform)
	}
	result.AuthorityFile = strings.TrimSpace(settings.AuthorityFile)
	if result.AuthorityFile == "" {
		result.AuthorityFile = defaultAuthorityFile(platform)
	}

	if result.AuthorityFile != "" {
		if _, err := os.Stat(result.AuthorityFile); err != nil {
			if result.XauthPath == "" {
				result.Reason = ReasonXauthMissing
				result.Detail = result.XauthPath
				return result, ErrUnavailable
			}

		}
	}
	if result.XauthPath == "" {
		result.Reason = ReasonXauthMissing
		return result, ErrUnavailable
	}
	if _, err := exec.LookPath(result.XauthPath); err != nil {
		result.Reason = ReasonXauthMissing
		result.Detail = filepath.Base(result.XauthPath)
		return result, ErrUnavailable
	}

	if entries, err := parseAuthorityFile(result.AuthorityFile); err == nil {
		if cookie, ok := pickCookie(entries, result.Display, result.Screen); ok {
			result.Cookie = cookie
			result.Available = true
			result.Reason = ReasonOK
			return result, nil
		}
	}
	cookie, reason := readCookie(ctx, result.XauthPath, result.AuthorityFile, result.Display)
	if cookie == "" {

		if reachableDisplaySocket(result.Display) {
			result.Cookie = fakeCookie(result.Screen)
			result.Available = true
			result.Reason = ReasonOK
			result.Notice = NoticeFakeCookie
			return result, nil
		}
		result.Reason = reason
		return result, ErrUnavailable
	}
	result.Cookie = cookie
	result.Available = true
	result.Reason = ReasonOK
	return result, nil
}

func readCookie(ctx context.Context, xauthPath, authorityFile, display string) (string, string) {
	args := make([]string, 0, 4)
	if authorityFile != "" {
		args = append(args, "-f", authorityFile)
	}
	args = append(args, "list", display)
	command := exec.CommandContext(ctx, xauthPath, args...)
	output, err := command.Output()
	if err != nil {
		return "", ReasonNoCookie
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))

		if len(fields) < 3 || fields[0] != display {
			continue
		}
		if !strings.EqualFold(fields[1], "MIT-MAGIC-COOKIE-1") {
			continue
		}
		return strings.TrimSpace(fields[2]), ReasonOK
	}
	return "", ReasonNoCookie
}

func reachableDisplaySocket(display string) bool {
	if !strings.HasPrefix(display, "/") {
		return false
	}
	info, err := os.Stat(display)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return false
	}
	conn, err := net.DialTimeout("unix", display, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func fakeCookie(screen int) string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		for index := range buffer {
			buffer[index] = byte(screen + index)
		}
	}
	return hexEncode(buffer)
}

func defaultDisplay(platform string) string {
	switch platform {
	case "darwin":

		if display := strings.TrimSpace(os.Getenv("DISPLAY")); display != "" {
			return display
		}
		if socket := launchdDisplaySocket(); socket != "" {
			return socket
		}
		return ":0"
	case "windows":
		return "127.0.0.1:0"
	default:
		if display := strings.TrimSpace(os.Getenv("DISPLAY")); display != "" {
			return display
		}
		return ":0"
	}
}

func defaultXauthPath(platform string) string {
	if platform == "darwin" {
		return "/opt/X11/bin/xauth"
	}
	if path, err := exec.LookPath("xauth"); err == nil {
		return path
	}
	return ""
}

func defaultAuthorityFile(platform string) string {
	if platform == "darwin" {
		if authority := strings.TrimSpace(os.Getenv("XAUTHORITY")); authority != "" {
			return authority
		}

		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".Xauthority")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
		}
		return launchdAuthorityFile()
	}
	if platform == "windows" {
		return ""
	}
	if authority := strings.TrimSpace(os.Getenv("XAUTHORITY")); authority != "" {
		return authority
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".Xauthority")
}

func launchdDisplaySocket() string {
	matches, err := filepath.Glob("/private/tmp/com.apple.launchd.*/org.xquartz:*")
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func launchdAuthorityFile() string {
	output, err := exec.Command("launchctl", "getenv", "XAUTHORITY").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func screenNumber(display string) int {
	_, screen, found := strings.Cut(display, ".")
	if !found {
		return 0
	}
	value := 0
	for _, char := range screen {
		if char < '0' || char > '9' {
			break
		}
		value = value*10 + int(char-'0')
	}
	return value
}
