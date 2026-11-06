package translate

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const baiduEndpoint = "https://fanyi-api.baidu.com/api/trans/vip/translate"

type baiduResponse struct {
	From         string `json:"from"`
	Translations []struct {
		Text string `json:"dst"`
	} `json:"trans_result"`
	ErrorCode json.RawMessage `json:"error_code"`
}

func (s *Service) baidu(ctx context.Context, text, target string, credentials Credentials) (Result, error) {
	appID := strings.TrimSpace(credentials.AppID)
	appKey := strings.TrimSpace(credentials.AppKey)
	if appID == "" || appKey == "" {
		return Result{}, errors.New("translate: baidu credentials are not configured")
	}
	salt, err := randomSalt()
	if err != nil {
		return Result{}, err
	}
	signBytes := md5.Sum([]byte(appID + text + salt + appKey))
	sign := hex.EncodeToString(signBytes[:])
	form := url.Values{}
	form.Set("q", text)
	form.Set("from", "auto")
	form.Set("to", baiduLanguage(target))
	form.Set("appid", appID)
	form.Set("salt", salt)
	form.Set("sign", sign)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baiduEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var response baiduResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{}, err
	}
	if code := rawJSONScalar(response.ErrorCode); code != "" && code != "0" {
		return Result{}, errors.New("translate: baidu returned an API error")
	}
	if len(response.Translations) == 0 {
		return Result{}, errors.New("translate: baidu returned an empty result")
	}
	parts := make([]string, 0, len(response.Translations))
	for _, item := range response.Translations {
		if translated := strings.TrimSpace(item.Text); translated != "" {
			parts = append(parts, translated)
		}
	}
	if len(parts) == 0 {
		return Result{}, errors.New("translate: baidu returned an empty translation")
	}
	return Result{Text: strings.Join(parts, "\n"), DetectedLanguage: strings.TrimSpace(response.From)}, nil
}

func baiduLanguage(language string) string {
	switch language {
	case "zh-CN", "zh_CN", "zh":
		return "zh"
	case "zh-TW", "zh_TW":
		return "cht"
	case "ja":
		return "jp"
	case "ko":
		return "kor"
	case "fr":
		return "fra"
	case "es":
		return "spa"
	default:
		return language
	}
}
