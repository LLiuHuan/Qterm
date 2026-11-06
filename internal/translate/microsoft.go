package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	microsoftTranslatorPage    = "https://cn.bing.com/translator"
	microsoftTranslateEndpoint = "https://cn.bing.com/ttranslatev3"
	microsoftReferer           = "https://cn.bing.com/translator"
	microsoftOrigin            = "https://cn.bing.com"
	microsoftAcceptLanguage    = "zh-CN,zh;q=0.9"
)

var (
	microsoftAbusePreventionPattern = regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*\[\s*(\d+)\s*,\s*["']([^"']+)["']\s*,\s*(\d+)\s*\]`)
	microsoftIGPattern              = regexp.MustCompile(`IG\s*:\s*["']([0-9A-Fa-f]{32})["']`)
	microsoftIIDPattern             = regexp.MustCompile(`data-iid\s*=\s*["']translator\.(\d+)["']`)
)

type microsoftResponse struct {
	DetectedLanguage *struct {
		Language string `json:"language"`
	} `json:"detectedLanguage"`
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
}

func (s *Service) microsoft(ctx context.Context, text, target string) (Result, error) {
	page, err := s.fetchMicrosoftTranslatorPage(ctx)
	if err != nil {
		return Result{}, microsoftFailure(ctx, err)
	}
	helper := microsoftAbusePreventionPattern.FindStringSubmatch(page)
	if len(helper) < 3 {
		return Result{}, errMicrosoftUnavailable
	}
	key, token := helper[1], helper[2]
	ig := firstRegexpGroup(microsoftIGPattern, page)
	iid := firstRegexpGroup(microsoftIIDPattern, page)
	if ig == "" || iid == "" {
		return Result{}, errMicrosoftUnavailable
	}

	form := url.Values{}
	form.Set("fromLang", "auto-detect")
	form.Set("to", microsoftLanguage(target))
	form.Set("text", text)
	form.Set("token", token)
	form.Set("key", key)

	endpoint := fmt.Sprintf("%s?isVertical=1&&IG=%s&IID=translator.%s.1", microsoftTranslateEndpoint, ig, iid)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, errMicrosoftUnavailable
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", browserUserAgent)
	request.Header.Set("Accept-Language", microsoftAcceptLanguage)
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Referer", microsoftReferer)
	request.Header.Set("Origin", microsoftOrigin)

	body, err := s.doWith(s.microsoftClient, request)
	if err != nil {
		return Result{}, microsoftFailure(ctx, err)
	}
	var responses []microsoftResponse
	if err := json.Unmarshal(body, &responses); err != nil {
		return Result{}, errMicrosoftUnavailable
	}
	if len(responses) == 0 || len(responses[0].Translations) == 0 {
		return Result{}, errMicrosoftUnavailable
	}
	translated := strings.TrimSpace(responses[0].Translations[0].Text)
	if translated == "" {
		return Result{}, errMicrosoftUnavailable
	}
	detected := ""
	if responses[0].DetectedLanguage != nil {
		detected = strings.TrimSpace(responses[0].DetectedLanguage.Language)
	}
	return Result{Text: translated, DetectedLanguage: detected}, nil
}

func (s *Service) fetchMicrosoftTranslatorPage(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, microsoftTranslatorPage, nil)
	if err != nil {
		return "", errMicrosoftUnavailable
	}
	request.Header.Set("User-Agent", browserUserAgent)
	request.Header.Set("Accept-Language", microsoftAcceptLanguage)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	body, err := s.doWith(s.microsoftClient, request)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func microsoftFailure(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errMicrosoftUnavailable
}

func firstRegexpGroup(pattern *regexp.Regexp, value string) string {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func microsoftLanguage(language string) string {
	switch language {
	case "zh-CN", "zh_CN":
		return "zh-Hans"
	case "zh-TW", "zh_TW":
		return "zh-Hant"
	default:
		return language
	}
}
