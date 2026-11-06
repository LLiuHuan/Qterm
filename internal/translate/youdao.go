package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const youdaoEndpoint = "https://openapi.youdao.com/api"

type youdaoResponse struct {
	ErrorCode   json.RawMessage `json:"errorCode"`
	Translation []string        `json:"translation"`
	Language    string          `json:"l"`
}

func (s *Service) youdao(ctx context.Context, text, target string, credentials Credentials) (Result, error) {
	appID := strings.TrimSpace(credentials.AppID)
	appKey := strings.TrimSpace(credentials.AppKey)
	if appID == "" || appKey == "" {
		return Result{}, errors.New("translate: youdao credentials are not configured")
	}
	salt, err := randomSalt()
	if err != nil {
		return Result{}, err
	}
	curtime := strconv.FormatInt(time.Now().Unix(), 10)
	signInput := appID + youdaoSignText(text) + salt + curtime + appKey
	signBytes := sha256.Sum256([]byte(signInput))
	form := url.Values{}
	form.Set("q", text)
	form.Set("from", "auto")
	form.Set("to", youdaoLanguage(target))
	form.Set("appKey", appID)
	form.Set("salt", salt)
	form.Set("sign", hex.EncodeToString(signBytes[:]))
	form.Set("signType", "v3")
	form.Set("curtime", curtime)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, youdaoEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var response youdaoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{}, err
	}
	if code := rawJSONScalar(response.ErrorCode); code != "" && code != "0" {
		return Result{}, errors.New("translate: youdao returned an API error")
	}
	if len(response.Translation) == 0 {
		return Result{}, errors.New("translate: youdao returned an empty result")
	}
	translated := strings.TrimSpace(response.Translation[0])
	if translated == "" {
		return Result{}, errors.New("translate: youdao returned an empty translation")
	}
	return Result{
		Text:             translated,
		DetectedLanguage: youdaoDetectedLanguage(response.Language),
	}, nil
}

func youdaoSignText(value string) string {
	runes := []rune(value)
	if len(runes) <= 20 {
		return value
	}
	return string(runes[:10]) + strconv.Itoa(len(runes)) + string(runes[len(runes)-10:])
}

func youdaoDetectedLanguage(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '2'); index > 0 {
		return value[:index]
	}
	return ""
}

func youdaoLanguage(language string) string {
	switch language {
	case "zh-CN", "zh_CN", "zh":
		return "zh-CHS"
	case "zh-TW", "zh_TW":
		return "zh-CHT"
	default:
		return language
	}
}
