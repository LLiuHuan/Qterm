package app

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/translate"
)

var errTranslateNotReady = errno.New(errno.CodeInvalidArg, "translate.service_unavailable", "")

var errTranslateCredentialsMissing = errno.New(errno.CodeInvalidArg, "translate.credentials_missing", "")

func (s *Service) TranslateText(text, target, provider, endpoint string) (translate.Result, error) {
	if s == nil {
		return translate.Result{}, publicError(errTranslateNotReady)
	}
	service, err := s.translationService()
	if err != nil {
		return translate.Result{}, publicError(err)
	}
	providerID := normalizeTranslationProvider(provider)
	if translationProviderNeedsCredentials(providerID) {
		credentials, credentialErr := s.readTranslationCredentials(context.Background(), providerID)
		if credentialErr != nil {
			if errors.Is(credentialErr, translate.ErrCredentialNotFound) {
				return translate.Result{}, publicError(errTranslateCredentialsMissing)
			}
			return translate.Result{}, publicError(credentialErr)
		}

		result, translateErr := service.TranslateWithCredentials(context.Background(), text, target, providerID, endpoint, credentials)
		if translateErr != nil {
			return translate.Result{}, publicError(translateErr)
		}
		return result, nil
	}
	result, err := service.Translate(context.Background(), text, target, providerID, endpoint)
	if err != nil {
		return translate.Result{}, publicError(err)
	}
	return result, nil
}

func (s *Service) TranslateLanguages() []string {
	return translate.Languages()
}

func (s *Service) ListTranslationProviderCredentials() ([]translate.CredentialStatus, error) {
	store, closeStore, err := s.openTranslationCredentialStore(context.Background())
	if err != nil {
		return []translate.CredentialStatus{}, publicError(err)
	}
	defer closeStore()
	statuses, err := store.Status(context.Background())
	if err != nil {
		return []translate.CredentialStatus{}, publicError(err)
	}
	return statuses, nil
}

func (s *Service) SaveTranslationProviderCredential(provider string, input translate.CredentialInput) error {
	providerID := normalizeTranslationProvider(provider)
	credentials := translate.Credentials{
		APIKey: input.APIKey,
		AppID:  input.AppID,
		AppKey: input.AppKey,
	}
	store, closeStore, err := s.openTranslationCredentialStore(context.Background())
	if err != nil {
		return publicError(err)
	}
	defer closeStore()
	if err := store.Put(context.Background(), providerID, credentials); err != nil {
		return publicError(err)
	}
	return nil
}

func (s *Service) DeleteTranslationProviderCredential(provider string) error {
	providerID := normalizeTranslationProvider(provider)
	store, closeStore, err := s.openTranslationCredentialStore(context.Background())
	if err != nil {
		return publicError(err)
	}
	defer closeStore()
	if err := store.Delete(context.Background(), providerID); err != nil {
		return publicError(err)
	}
	return nil
}

func (s *Service) translationService() (*translate.Service, error) {
	if s == nil {
		return nil, errTranslateNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.translate == nil {
		s.translate = translate.NewService()
	}
	return s.translate, nil
}

func (s *Service) readTranslationCredentials(ctx context.Context, provider translate.Provider) (translate.Credentials, error) {
	store, closeStore, err := s.openTranslationCredentialStore(ctx)
	if err != nil {
		return translate.Credentials{}, err
	}
	defer closeStore()
	return store.Get(ctx, provider)
}

func (s *Service) openTranslationCredentialStore(ctx context.Context) (*translate.CredentialStore, func(), error) {
	if s == nil || s.security == nil {
		return nil, func() {}, errTranslateNotReady
	}

	s.translationCredentialsMu.Lock()
	key, err := s.security.DataKey(ctx)
	if err != nil {
		s.translationCredentialsMu.Unlock()
		return nil, func() {}, errTranslateNotReady
	}
	defer clear(key)
	store, err := translate.OpenCredentialStore(s.security.Root(), key)
	if err != nil {
		s.translationCredentialsMu.Unlock()
		return nil, func() {}, err
	}
	return store, s.translationCredentialsMu.Unlock, nil
}

func normalizeTranslationProvider(provider string) translate.Provider {
	return translate.Provider(strings.ToLower(strings.TrimSpace(provider)))
}

func translationProviderNeedsCredentials(provider translate.Provider) bool {
	switch provider {
	case translate.ProviderDeepL, translate.ProviderBaidu, translate.ProviderAli, translate.ProviderYoudao:
		return true
	default:
		return false
	}
}
