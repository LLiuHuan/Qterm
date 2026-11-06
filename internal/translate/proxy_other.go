//go:build !darwin

package translate

import (
	"net/http"
	"net/url"
)

func proxyForRequest(request *http.Request) (*url.URL, error) {
	return http.ProxyFromEnvironment(request)
}
