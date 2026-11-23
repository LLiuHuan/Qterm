package clitool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"changeme/internal/preferences"
)

const CommandName = "qterm-cli"

const (
	skillFolder = "qterm"

	skillFileName = "SKILL.md"
)

const preferenceKey = "qterm.cliTool.v1"

const (
	HintBinaryMissing = "binaryMissing"

	HintUnsupportedPlatform = "unsupportedPlatform"

	HintNotOnPath = "notOnPath"

	HintRestartShell = "restartShell"

	HintPackageManaged = "packageManaged"
)

const (
	ExecGuardOff = "off"

	ExecGuardRules = "rules"

	ExecGuardStrict = "strict"
)

type Settings struct {
	ExecGuard string `json:"execGuard"`
}

func DefaultSettings() Settings {
	return Settings{ExecGuard: ExecGuardRules}
}

func NormalizeSettings(value Settings) Settings {
	switch value.ExecGuard {
	case ExecGuardOff, ExecGuardRules, ExecGuardStrict:
		return Settings{ExecGuard: value.ExecGuard}
	default:
		return DefaultSettings()
	}
}

type State struct {
	Platform string `json:"platform"`

	Supported bool `json:"supported"`

	BinaryPath      string `json:"binaryPath"`
	BinaryAvailable bool   `json:"binaryAvailable"`

	InstallDir string `json:"installDir"`
	LinkPath   string `json:"linkPath"`

	Installed bool `json:"installed"`

	Removable bool `json:"removable"`

	OnPath bool `json:"onPath"`

	SkillsPath      string `json:"skillsPath"`
	SkillsInstalled bool   `json:"skillsInstalled"`

	Settings Settings `json:"settings"`

	HintKey string `json:"hintKey,omitempty"`
}

type Service struct {
	prefs *preferences.Store

	mu       sync.Mutex
	settings Settings
	loaded   bool

	execPath           string
	homeDir            string
	pathEnv            string
	installDirOverride string
}

func NewService(prefs *preferences.Store) *Service {
	return &Service{prefs: prefs, settings: DefaultSettings()}
}

func (s *Service) Settings() (Settings, error) {
	if s == nil {
		return DefaultSettings(), nil
	}
	if err := s.load(); err != nil {
		return NormalizeSettings(s.settingsSnapshot()), err
	}
	return s.settingsSnapshot(), nil
}

func (s *Service) ApplySettings(value Settings) (Settings, error) {
	if s == nil {
		return DefaultSettings(), errors.New("cli tool service is not ready")
	}
	normalized := NormalizeSettings(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prefs != nil {
		document, marshalErr := json.Marshal(normalized)
		if marshalErr != nil {
			return s.settings, fmt.Errorf("encode cli tool settings: %w", marshalErr)
		}
		if err := s.prefs.Set(context.Background(), preferenceKey, string(document)); err != nil {
			return s.settings, fmt.Errorf("save cli tool settings: %w", err)
		}
	}
	s.settings = normalized
	s.loaded = true
	return normalized, nil
}

func (s *Service) State() (State, error) {
	if s == nil {
		return State{}, errors.New("cli tool service is not ready")
	}
	settings, err := s.Settings()

	binary, available := s.binaryPath()
	installDir := s.installDir()
	linkPath := filepath.Join(installDir, commandFileName())

	ownEntry := entryInstalled(binary, linkPath) && !(binary != "" && samePath(linkPath, binary))
	managedByPath := available && s.binaryOnPath(binary)
	installed := ownEntry || managedByPath
	onPath := s.dirOnPath(installDir) || managedByPath
	skillsPath := s.skillsPath()
	skillsInstalled := fileExists(skillsPath)

	state := State{
		Platform:        runtime.GOOS,
		Supported:       true,
		BinaryPath:      binary,
		BinaryAvailable: available,
		InstallDir:      installDir,
		LinkPath:        linkPath,
		Installed:       installed,
		Removable:       ownEntry,
		OnPath:          onPath,
		SkillsPath:      skillsPath,
		SkillsInstalled: skillsInstalled,
		Settings:        settings,
	}

	switch {
	case !available:
		state.HintKey = HintBinaryMissing
	case !state.Supported:
		state.HintKey = HintUnsupportedPlatform
	case installed && !state.Removable:

		state.HintKey = HintPackageManaged
	case installed && !onPath:
		state.HintKey = HintNotOnPath
	case managesSearchPath() && installed && !pathContains(processPath(), installDir):

		state.HintKey = HintRestartShell
	}
	return state, err
}

func (s *Service) InstallBinary() (State, error) {
	if s == nil {
		return State{}, errors.New("cli tool service is not ready")
	}
	binary, available := s.binaryPath()
	if !available {
		return s.StateOrZero(), newError("settings.cli.error.binaryMissing", binary)
	}
	if s.binaryOnPath(binary) {

		return s.State()
	}
	installDir := s.installDir()
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.dirNotWritable", installDir)
	}
	linkPath := filepath.Join(installDir, commandFileName())
	if ours, exists := entryOwned(linkPath); exists {

		if !ours {
			return s.StateOrZero(), newError("settings.cli.error.nameTaken", linkPath)
		}
		if err := removeEntry(linkPath); err != nil {
			return s.StateOrZero(), newError("settings.cli.error.installFailed", linkPath)
		}
	}
	if err := installEntry(binary, linkPath); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.installFailed", linkPath)
	}

	if err := ensureSearchPath(installDir); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.pathFailed", installDir)
	}
	state, err := s.State()
	if err != nil {
		return state, err
	}
	if !state.Installed {
		return state, newError("settings.cli.error.installFailed", linkPath)
	}
	return state, nil
}

func (s *Service) UninstallBinary() (State, error) {
	if s == nil {
		return State{}, errors.New("cli tool service is not ready")
	}
	binary, _ := s.binaryPath()
	installDir := s.installDir()
	linkPath := filepath.Join(installDir, commandFileName())
	if ours, exists := entryOwned(linkPath); exists {
		if !ours {

			if s.binaryOnPath(binary) {
				return s.State()
			}
			return s.StateOrZero(), newError("settings.cli.error.nameTaken", linkPath)
		}
		if err := removeEntry(linkPath); err != nil {
			return s.StateOrZero(), newError("settings.cli.error.uninstallFailed", linkPath)
		}
	}

	if err := removeSearchPath(installDir); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.pathFailed", installDir)
	}
	return s.State()
}

func (s *Service) InstallSkills() (State, error) {
	if s == nil {
		return State{}, errors.New("cli tool service is not ready")
	}
	path := s.skillsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.skillsFailed", path)
	}
	if err := os.WriteFile(path, skillDocument, 0o644); err != nil {
		return s.StateOrZero(), newError("settings.cli.error.skillsFailed", path)
	}
	return s.State()
}

func (s *Service) UninstallSkills() (State, error) {
	if s == nil {
		return State{}, errors.New("cli tool service is not ready")
	}
	path := s.skillsPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return s.StateOrZero(), newError("settings.cli.error.skillsFailed", path)
	}

	_ = os.Remove(filepath.Dir(path))
	return s.State()
}

func (s *Service) StateOrZero() State {
	state, _ := s.State()
	return state
}

func (s *Service) settingsSnapshot() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return NormalizeSettings(s.settings)
}

func (s *Service) load() error {
	s.mu.Lock()
	if s.loaded {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	settings := DefaultSettings()
	var loadErr error
	if s.prefs != nil {
		raw, found, err := s.prefs.Get(context.Background(), preferenceKey)
		switch {
		case err != nil:
			loadErr = fmt.Errorf("load cli tool settings: %w", err)
		case found:
			if parsed, ok := parseSettings(raw); ok {
				settings = parsed
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.settings = settings
		s.loaded = true
	}
	return loadErr
}

func parseSettings(raw string) (Settings, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Settings{}, false
	}
	var stored Settings
	if err := json.Unmarshal([]byte(trimmed), &stored); err != nil {
		return Settings{}, false
	}
	if stored.ExecGuard == "" {
		return Settings{}, false
	}
	return NormalizeSettings(stored), true
}

type skillError struct {
	key    string
	detail string
}

func (e *skillError) Error() string { return e.key }

func (e *skillError) MessageKey() string { return e.key }

func (e *skillError) Detail() string { return e.detail }

func newError(key, detail string) error { return &skillError{key: key, detail: detail} }

func commandFileName() string {
	if runtime.GOOS == "windows" {
		return CommandName + ".exe"
	}
	return CommandName
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ownLink(path string) (owned bool, exists bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, !errors.Is(err, os.ErrNotExist)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, true
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false, true
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	if _, statErr := os.Stat(target); statErr != nil {

		return true, true
	}
	return filepath.Base(target) == commandFileName(), true
}

func linkPointsTo(path, target string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	wantResolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		wantResolved = target
	}
	return resolved == wantResolved
}
