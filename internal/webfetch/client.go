package webfetch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	maxBodyBytes = 2 << 20

	maxRedirects = 5

	userAgent = "Qterm-Agent/1.0"
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func isBlockedAddr(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if !addr.IsValid() {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func newClient(settings Settings) (*http.Client, error) {
	proxy, err := proxyFunc(settings)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:                 proxy,
		DialContext:           guardedDial(settings.AllowPrivate),
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	client := &http.Client{
		Timeout:   fetchTimeout(),
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrInvalidURL
			}
			return nil
		},
	}
	return client, nil
}

func fetchTimeout() time.Duration {
	return 15 * time.Second
}

func proxyFunc(settings Settings) (func(*http.Request) (*url.URL, error), error) {
	switch settings.Mode {
	case ModeDirect:
		return nil, nil
	case ModeCustom:
		parsed, err := url.Parse(settings.ProxyURL)
		if err != nil || parsed.Host == "" {
			return nil, ErrInvalidURL
		}
		return http.ProxyURL(parsed), nil
	default:
		return http.ProxyFromEnvironment, nil
	}
}

func guardedDial(allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrUpstream
		}

		if literal, parseErr := netip.ParseAddr(host); parseErr == nil {
			if !allowPrivate && isBlockedAddr(literal) {
				return nil, ErrBlockedAddress
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(literal.String(), port))
		}
		addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addrs) == 0 {
			return nil, ErrUpstream
		}
		if !allowPrivate {
			for _, addr := range addrs {
				if isBlockedAddr(addr) {
					return nil, ErrBlockedAddress
				}
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].String(), port))
	}
}

func fetchWithClient(ctx context.Context, client *http.Client, target *url.URL) (Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Result{}, ErrInvalidURL
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,application/json;q=0.8,*/*;q=0.1")

	response, err := client.Do(request)
	if err != nil {
		return Result{}, classifyRequestError(err)
	}
	defer func() { _ = response.Body.Close() }()

	contentType := primaryContentType(response.Header.Get("Content-Type"))
	if !readableContentType(contentType) {
		return Result{}, ErrUnsupportedContent
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return Result{}, ErrUpstream
	}
	truncated := len(data) > maxBodyBytes
	if truncated {
		data = data[:maxBodyBytes]
	}
	text, title := extractText(contentType, data)
	return Result{
		URL:         response.Request.URL.String(),
		Status:      response.StatusCode,
		ContentType: contentType,
		Title:       title,
		Text:        text,
		Truncated:   truncated,
		Bytes:       int64(len(data)),
	}, nil
}

func classifyRequestError(err error) error {
	switch {
	case errors.Is(err, ErrBlockedAddress), errors.Is(err, ErrTooManyRedirects), errors.Is(err, ErrInvalidURL):
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return err
	default:
		return ErrUpstream
	}
}

func primaryContentType(raw string) string {
	value := strings.TrimSpace(raw)
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	return strings.ToLower(value)
}

func readableContentType(contentType string) bool {
	switch {
	case strings.HasPrefix(contentType, "text/"):
		return true
	case contentType == "application/json", contentType == "application/xml",
		contentType == "application/xhtml+xml", contentType == "application/ld+json":
		return true
	default:
		return false
	}
}
