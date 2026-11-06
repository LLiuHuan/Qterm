package translate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func randomSalt() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", errors.New("translate: unable to create request salt")
	}
	return hex.EncodeToString(value[:]), nil
}

func rawJSONScalar(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var stringValue string
	if json.Unmarshal(raw, &stringValue) == nil {
		return strings.TrimSpace(stringValue)
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func newTranslateTransport() *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {

		return &http.Transport{Proxy: proxyForRequest}
	}
	cloned := transport.Clone()
	cloned.Proxy = proxyForRequest
	return cloned
}
