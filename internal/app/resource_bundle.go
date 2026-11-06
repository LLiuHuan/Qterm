package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"changeme/internal/assetdata"
	"changeme/internal/resourcebundle"
)

const resourceAssetsEntry = "assets/assets.json"

const maxResourceBundleBytes int64 = 64 << 20

func (s *Service) ExportResourceBundle(ctx context.Context, password string) (bool, error) {
	if s == nil || s.fileDialogs == nil || s.hosts == nil || s.credentials == nil {
		return false, errors.New("resource export is not ready")
	}
	if ctx == nil {
		return false, errors.New("resource export context is nil")
	}
	if err := validateResourceBundlePassword(password); err != nil {
		return false, publicError(err)
	}
	path, err := s.fileDialogs.ChooseResourceBundleDestination()
	if err != nil {
		return false, publicError(err)
	}
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	assets, err := s.hosts.ExportAssets(ctx)
	if err != nil {
		return false, publicError(err)
	}
	credentials, err := s.credentials.ExportAssets(ctx)
	if err != nil {
		assets.Clear()
		return false, publicError(err)
	}
	assets.Credentials = credentials
	assets.CreatedAt = time.Now().UTC()
	defer assets.Clear()
	if err := assets.Validate(); err != nil {
		return false, publicError(err)
	}
	payload, err := json.Marshal(assets)
	if err != nil {
		return false, publicError(fmt.Errorf("encode resource assets: %w", err))
	}
	defer clearBytes(payload)
	manifest := resourcebundle.Manifest{Scope: "assets", CreatedAt: assets.CreatedAt}
	if err := resourcebundle.Write(ctx, path, []byte(password), manifest, []resourcebundle.Entry{{Name: resourceAssetsEntry, Data: payload}}); err != nil {
		return false, publicError(err)
	}
	return true, nil
}

func (s *Service) ImportResourceBundle(ctx context.Context, password string) (bool, error) {
	if s == nil || s.fileDialogs == nil || s.hosts == nil || s.credentials == nil {
		return false, errors.New("resource import is not ready")
	}
	if ctx == nil {
		return false, errors.New("resource import context is nil")
	}
	if err := validateResourceBundlePassword(password); err != nil {
		return false, publicError(err)
	}
	path, err := s.fileDialogs.ChooseResourceBundleFile()
	if err != nil {
		return false, publicError(err)
	}
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, publicError(err)
	}
	if err := validateResourceBundleFile(info); err != nil {
		return false, publicError(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, publicError(err)
	}
	defer clearBytes(data)
	return s.importResourceBundleData(ctx, data, password)
}

func (s *Service) ValidateResourceBundleData(ctx context.Context, data []byte, password string) (bool, error) {
	if err := validateResourceBundlePassword(password); err != nil {
		return false, publicError(err)
	}
	if err := validateResourceBundleDataSize(data); err != nil {
		return false, publicError(err)
	}
	manifest, entries, err := resourcebundle.ExtractDataBytes(ctx, data, []byte(password))
	if err != nil {
		return false, publicError(err)
	}
	defer func() {
		for name, value := range entries {
			clear(value)
			delete(entries, name)
		}
	}()
	if manifest.Scope != "assets" {
		return false, publicError(errors.New("unsupported resource bundle scope"))
	}
	if _, ok := entries[resourceAssetsEntry]; !ok {
		return false, publicError(errors.New("resource assets entry is missing"))
	}
	return true, nil
}

func (s *Service) ImportResourceBundleData(ctx context.Context, data []byte, password string) (bool, error) {
	if s == nil || s.hosts == nil || s.credentials == nil {
		return false, errors.New("resource import is not ready")
	}
	if ctx == nil {
		return false, errors.New("resource import context is nil")
	}
	if err := validateResourceBundlePassword(password); err != nil {
		return false, publicError(err)
	}
	if err := validateResourceBundleDataSize(data); err != nil {
		return false, publicError(err)
	}
	return s.importResourceBundleData(ctx, data, password)
}

func (s *Service) importResourceBundleData(ctx context.Context, data []byte, password string) (bool, error) {
	s.assetMu.Lock()
	defer s.assetMu.Unlock()
	manifest, entries, err := resourcebundle.ExtractDataBytes(ctx, data, []byte(password))
	if err != nil {
		return false, publicError(err)
	}
	defer func() {
		for name, value := range entries {
			clear(value)
			delete(entries, name)
		}
	}()
	if manifest.Scope != "assets" {
		return false, publicError(errors.New("unsupported resource bundle scope"))
	}
	payload, ok := entries[resourceAssetsEntry]
	if !ok {
		return false, publicError(errors.New("resource assets entry is missing"))
	}
	var assets assetdata.Snapshot
	if err := json.Unmarshal(payload, &assets); err != nil {
		return false, publicError(errors.New("invalid resource assets"))
	}
	defer assets.Clear()
	if err := assets.Validate(); err != nil {
		return false, publicError(err)
	}
	previousHosts, err := s.hosts.ExportAssets(ctx)
	if err != nil {
		return false, publicError(err)
	}
	previousCredentials, err := s.credentials.ExportAssets(ctx)
	if err != nil {
		previousHosts.Clear()
		return false, publicError(err)
	}
	defer previousHosts.Clear()
	defer func() {
		for index := range previousCredentials {
			clear(previousCredentials[index].Value)
		}
	}()
	previousHosts.Credentials = previousCredentials
	restore := func() error {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.credentials.ReplaceAssets(restoreCtx, previousCredentials); err != nil {
			return fmt.Errorf("restore credentials: %w", err)
		}
		if err := s.hosts.ReplaceAssets(restoreCtx, previousHosts); err != nil {
			return fmt.Errorf("restore hosts: %w", err)
		}
		return nil
	}
	if err := s.credentials.ReplaceAssets(ctx, assets.Credentials); err != nil {
		if restoreErr := restore(); restoreErr != nil {
			return false, publicError(errors.Join(err, restoreErr))
		}
		return false, publicError(err)
	}
	if err := s.hosts.ReplaceAssets(ctx, assets); err != nil {
		if restoreErr := restore(); restoreErr != nil {
			return false, publicError(errors.Join(err, restoreErr))
		}
		return false, publicError(err)
	}
	return true, nil
}

func validateResourceBundlePassword(password string) error {
	if len(password) < 8 || len(password) > 4096 {
		return errors.New("resource bundle password length is invalid")
	}
	return nil
}

func validateResourceBundleDataSize(data []byte) error {
	if len(data) == 0 {
		return errors.New("resource bundle is empty")
	}
	if int64(len(data)) > maxResourceBundleBytes {
		return errors.New("resource bundle is too large")
	}
	return nil
}

func validateResourceBundleFile(info os.FileInfo) error {
	if info == nil || !info.Mode().IsRegular() || info.Size() > maxResourceBundleBytes {
		return errors.New("resource bundle is invalid or too large")
	}
	if info.Size() == 0 {
		return errors.New("resource bundle is empty")
	}
	return nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
