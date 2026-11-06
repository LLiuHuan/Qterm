//go:build darwin

package translate

import (
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const systemProxyTTL = 60 * time.Second

var (
	systemProxyMu       sync.Mutex
	systemProxyCached   *url.URL
	systemProxyBypassed []string
	systemProxyExpires  time.Time
)

func proxyForRequest(request *http.Request) (*url.URL, error) {
	if envProxy, err := http.ProxyFromEnvironment(request); err != nil || envProxy != nil {
		return envProxy, err
	}
	proxy, bypassed := cachedSystemProxy()
	if proxy == nil {
		return nil, nil
	}
	if proxyBypassedHost(request.URL.Hostname(), bypassed) {
		return nil, nil
	}
	return proxy, nil
}

func cachedSystemProxy() (*url.URL, []string) {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()
	if time.Now().Before(systemProxyExpires) {
		return systemProxyCached, systemProxyBypassed
	}
	proxy, bypassed := readSystemProxy()
	systemProxyCached, systemProxyBypassed = proxy, bypassed
	systemProxyExpires = time.Now().Add(systemProxyTTL)
	return proxy, bypassed
}

func readSystemProxy() (*url.URL, []string) {
	output, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return nil, nil
	}
	values := parseScutilProxy(string(output))
	bypassed := parseScutilExceptions(values)
	httpsEnabled := values["HTTPSEnable"] == "1"
	httpEnabled := values["HTTPEnable"] == "1"
	switch {
	case httpsEnabled && values["HTTPSProxy"] != "" && values["HTTPSPort"] != "":
		return buildProxyURL("http", values["HTTPSProxy"], values["HTTPSPort"]), bypassed
	case httpEnabled && values["HTTPProxy"] != "" && values["HTTPPort"] != "":
		return buildProxyURL("http", values["HTTPProxy"], values["HTTPPort"]), bypassed
	default:
		return nil, bypassed
	}
}

func buildProxyURL(scheme, host, port string) *url.URL {
	if _, err := strconv.Atoi(port); err != nil {
		return nil
	}
	return &url.URL{Scheme: scheme, Host: net.JoinHostPort(strings.TrimSpace(host), port)}
}

func parseScutilProxy(output string) map[string]string {
	values := make(map[string]string, 8)
	stack := make([]string, 0, 4)
	currentArray := ""
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || line == "<dictionary> {" || line == "<array> {":
			continue
		case line == "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			currentArray = ""
			continue
		case strings.HasSuffix(line, "<array> {"):
			key := strings.TrimSpace(strings.TrimSuffix(line, "<array> {"))
			key = strings.TrimSuffix(key, ":")
			if key != "" && len(stack) == 0 {
				currentArray = key
				stack = append(stack, key)
			}
			continue
		case strings.Contains(line, " : "):
			if len(stack) > 0 {

				key, value, _ := strings.Cut(line, " : ")
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if currentArray != "" {
					values[currentArray+"."+key] = value
				}
			} else {
				key, value, _ := strings.Cut(line, " : ")
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key != "" {
					values[key] = value
				}
			}
		}
	}
	return values
}

func parseScutilExceptions(values map[string]string) []string {
	result := make([]string, 0, 8)
	for index := 0; index < 32; index++ {
		value, ok := values["ExceptionsList."+strconv.Itoa(index)]
		if !ok {
			break
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func proxyBypassedHost(host string, excepted []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	for _, entry := range excepted {
		pattern := strings.ToLower(strings.TrimSpace(entry))
		if pattern == "" {
			continue
		}
		switch {
		case pattern == "<local>":
			if !strings.Contains(host, ".") {
				return true
			}
		case strings.HasPrefix(pattern, "*."):
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(host, suffix) {
				return true
			}
		case strings.Contains(pattern, "/"):
			if ip != nil {
				if _, network, err := net.ParseCIDR(pattern); err == nil && network.Contains(ip) {
					return true
				}
			}
		default:
			if host == pattern {
				return true
			}
			if ip != nil {
				if patternIP := net.ParseIP(pattern); patternIP != nil && patternIP.Equal(ip) {
					return true
				}
			}
		}
	}
	return false
}
