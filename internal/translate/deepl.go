package translate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type deeplResponse struct {
	Translations []struct {
		Text                   string `json:"text"`
		DetectedSourceLanguage string `json:"detected_source_language"`
	} `json:"translations"`
}

func (s *Service) deepl(ctx context.Context, text, target string, credentials Credentials) (Result, error) {
	apiKey := strings.TrimSpace(credentials.APIKey)
	if apiKey == "" {
		return Result{}, errors.New("translate: deepl credentials are not configured")
	}
	base := "https://api.deepl.com"
	if strings.HasSuffix(apiKey, ":fx") {
		base = "https://api-free.deepl.com"
	}
	form := url.Values{}
	form.Set("text", text)
	form.Set("target_lang", deeplLanguage(target))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v2/translate", strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Authorization", "DeepL-Auth-Key "+apiKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var response deeplResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{}, err
	}
	if len(response.Translations) == 0 {
		return Result{}, errors.New("translate: deepl returned an empty result")
	}
	translation := strings.TrimSpace(response.Translations[0].Text)
	if translation == "" {
		return Result{}, errors.New("translate: deepl returned an empty translation")
	}
	return Result{
		Text:             translation,
		DetectedLanguage: strings.TrimSpace(response.Translations[0].DetectedSourceLanguage),
	}, nil
}

func deeplLanguage(language string) string {
	switch language {
	case "zh-CN", "zh_CN", "zh":
		return "ZH-HANS"
	case "zh-TW", "zh_TW":
		return "ZH-HANT"
	case "en":
		return "EN"
	case "ja":
		return "JA"
	case "ko":
		return "KO"
	case "fr":
		return "FR"
	case "de":
		return "DE"
	case "es":
		return "ES"
	case "pt":
		return "PT-BR"
	case "ru":
		return "RU"
	case "it":
		return "IT"
	default:
		return language
	}
}
