package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"changeme/internal/preferences"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const singleInstanceUniqueID = "com.qterm.app"

var singleInstanceEnabled atomic.Bool

func readSettingsBlob(configDir, dbName string) string {
	path := filepath.Join(configDir, "Qterm", dbName)
	store, err := preferences.Open(context.Background(), path)
	if err != nil {
		slog.Warn("preference store is not readable before startup", "error", err)
		return ""
	}
	defer func() { _ = store.Close() }()
	raw, found, err := store.Get(context.Background(), settingsPreferenceKey)
	if err != nil || !found {
		return ""
	}
	return raw
}

const settingsPreferenceKey = "qterm.settings.v1"

func newSingleInstanceOptions(configDir string, focus func(), onArgs func([]string)) *application.SingleInstanceOptions {
	enabled := true
	if raw := readSettingsBlob(configDir, "preferences.db"); raw != "" {
		var settings struct {
			SingleInstance *bool `json:"singleInstance"`
		}
		if json.Unmarshal([]byte(raw), &settings) == nil && settings.SingleInstance != nil {
			enabled = *settings.SingleInstance
		}
	}
	singleInstanceEnabled.Store(enabled)
	if !enabled {
		return nil
	}
	return &application.SingleInstanceOptions{
		UniqueID: singleInstanceUniqueID,

		OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
			if focus != nil {
				focus()
			}

			if onArgs != nil && len(data.Args) > 0 {
				onArgs(data.Args)
			}
		},

		ExitCode: 0,
	}
}

func applySingleInstancePreference(enabled bool) {
	singleInstanceEnabled.Store(enabled)
}

func (s *Service) SingleInstanceEnabled() bool {
	return singleInstanceEnabled.Load()
}
