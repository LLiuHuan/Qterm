package translate

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const aliEndpoint = "https://mt.aliyuncs.com/"

type aliResponse struct {
	Code json.RawMessage `json:"Code"`
	Data *struct {
		Translated       string `json:"Translated"`
		DetectedLanguage string `json:"DetectedLanguage"`
	} `json:"Data"`
}

func (s *Service) ali(ctx context.Context, text, target string, credentials Credentials) (Result, error) {
	accessKeyID := strings.TrimSpace(credentials.AppID)
	accessKeySecret := strings.TrimSpace(credentials.AppKey)
	if accessKeyID == "" || accessKeySecret == "" {
		return Result{}, errors.New("translate: ali credentials are not configured")
	}
	salt, err := randomSalt()
	if err != nil {
		return Result{}, err
	}
	form := url.Values{}
	form.Set("FormatType", "text")
	form.Set("SourceLanguage", "auto")
	form.Set("TargetLanguage", aliLanguage(target))
	form.Set("SourceText", text)
	form.Set("Scene", "general")
	body := form.Encode()
	bodyHashBytes := sha256.Sum256([]byte(body))
	bodyHash := hex.EncodeToString(bodyHashBytes[:])
	date := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	signedHeaders := "host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version"
	canonicalHeaders := strings.Join([]string{
		"host:mt.aliyuncs.com",
		"x-acs-action:TranslateGeneral",
		"x-acs-content-sha256:" + bodyHash,
		"x-acs-date:" + date,
		"x-acs-signature-nonce:" + salt,
		"x-acs-version:2018-10-12",
	}, "\n")
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n\n" + signedHeaders + "\n" + bodyHash
	hashedRequestBytes := sha256.Sum256([]byte(canonicalRequest))
	hashedRequest := hex.EncodeToString(hashedRequestBytes[:])
	stringToSign := "ACS3-HMAC-SHA256\n" + hashedRequest
	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	_, _ = mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))
	authorization := "ACS3-HMAC-SHA256 Credential=" + accessKeyID + ",SignedHeaders=" + signedHeaders + ",Signature=" + signature

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, aliEndpoint, strings.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("x-acs-action", "TranslateGeneral")
	request.Header.Set("x-acs-version", "2018-10-12")
	request.Header.Set("x-acs-content-sha256", bodyHash)
	request.Header.Set("x-acs-date", date)
	request.Header.Set("x-acs-signature-nonce", salt)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bodyBytes, err := s.do(request)
	if err != nil {
		return Result{}, err
	}
	var response aliResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return Result{}, err
	}
	if code := rawJSONScalar(response.Code); code != "" && code != "200" {
		return Result{}, errors.New("translate: ali returned an API error")
	}
	if response.Data == nil {
		return Result{}, errors.New("translate: ali returned an empty result")
	}
	translated := strings.TrimSpace(response.Data.Translated)
	if translated == "" {
		return Result{}, errors.New("translate: ali returned an empty translation")
	}
	return Result{Text: translated, DetectedLanguage: strings.TrimSpace(response.Data.DetectedLanguage)}, nil
}

func aliLanguage(language string) string {
	switch language {
	case "zh-CN", "zh_CN", "zh":
		return "zh"
	case "zh-TW", "zh_TW":
		return "zh-tw"
	default:
		return language
	}
}
