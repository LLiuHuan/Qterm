package applock

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"golang.org/x/crypto/argon2"
)

const (
	minPasswordLen = 4
	maxPasswordLen = 128

	maxAutoLockMinutes = 24 * 60
)

const (
	defaultTime    uint32 = 3
	defaultMemory  uint32 = 64 * 1024
	defaultThreads uint8  = 4
	defaultKeyLen  uint32 = 32
	saltLen               = 16
)

type Status struct {
	Enabled bool `json:"enabled"`

	HasPassword bool `json:"hasPassword"`

	AutoLockMinutes int `json:"autoLockMinutes"`

	HasRecovery      bool   `json:"hasRecovery"`
	RecoveryQuestion string `json:"recoveryQuestion"`
}

type fileState struct {
	Version         int    `json:"version"`
	Enabled         bool   `json:"enabled"`
	AutoLockMinutes int    `json:"autoLockMinutes"`
	Salt            string `json:"salt"`
	Hash            string `json:"hash"`
	Time            uint32 `json:"time"`
	MemoryKiB       uint32 `json:"memoryKiB"`
	Threads         uint8  `json:"threads"`
	KeyLen          uint32 `json:"keyLen"`

	Question   string `json:"question,omitempty"`
	AnswerSalt string `json:"answerSalt,omitempty"`
	AnswerHash string `json:"answerHash,omitempty"`
}

type Manager struct {
	mu   sync.Mutex
	path string
	data fileState
}

func New(root string) (*Manager, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, errno.New(errno.CodeInvalidArg, "applock.invalidPath", "")
	}
	manager := &Manager{path: filepath.Join(filepath.Clean(root), "app-lock.json")}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) load() error {
	raw, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "applock.readFailed", err)
	}
	var state fileState
	if err := json.Unmarshal(raw, &state); err != nil {

		m.data = fileState{}
		return nil
	}
	m.data = state
	return nil
}

func (m *Manager) Status() Status {
	if m == nil {
		return Status{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	enabled := m.data.Enabled && m.data.Hash != ""
	return Status{
		Enabled:          enabled,
		HasPassword:      m.data.Hash != "",
		AutoLockMinutes:  m.data.AutoLockMinutes,
		HasRecovery:      m.data.AnswerHash != "",
		RecoveryQuestion: m.data.Question,
	}
}

func (m *Manager) Enable(password string, autoLockMinutes int, question, answer string) error {
	if m == nil {
		return errno.New(errno.CodeUnknown, "applock.notReady", "")
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if err := validateMinutes(autoLockMinutes); err != nil {
		return err
	}
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	if (question == "") != (answer == "") {
		return errno.New(errno.CodeInvalidArg, "applock.incompleteRecovery", "")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data.Enabled && m.data.Hash != "" {
		return errno.New(errno.CodePermissionDenied, "applock.alreadyEnabled", "")
	}
	return m.saveLocked(password, autoLockMinutes, true, question, answer)
}

func (m *Manager) ChangePassword(current, next string) error {
	if m == nil {
		return errno.New(errno.CodeUnknown, "applock.notReady", "")
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data.Hash != "" && !m.verifyLocked(current) {
		return errno.New(errno.CodePermissionDenied, "applock.wrongPassword", "")
	}
	return m.saveLocked(next, m.data.AutoLockMinutes, true, "", "")
}

func (m *Manager) Disable(password string) error {
	if m == nil {
		return errno.New(errno.CodeUnknown, "applock.notReady", "")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data.Hash != "" && !m.verifyLocked(password) {
		return errno.New(errno.CodePermissionDenied, "applock.wrongPassword", "")
	}
	m.data = fileState{Version: 1, AutoLockMinutes: 0}
	return m.persistLocked()
}

func (m *Manager) ResetPasswordByRecovery(answer, next string) error {
	if m == nil {
		return errno.New(errno.CodeUnknown, "applock.notReady", "")
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data.AnswerHash == "" || m.data.AnswerSalt == "" {
		return errno.New(errno.CodePermissionDenied, "applock.noRecovery", "")
	}
	if !m.verifyAnswerLocked(answer) {
		return errno.New(errno.CodePermissionDenied, "applock.wrongAnswer", "")
	}
	return m.saveLocked(next, m.data.AutoLockMinutes, true, "", "")
}

func (m *Manager) verifyAnswerLocked(answer string) bool {
	if m.data.AnswerHash == "" || strings.TrimSpace(answer) == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(m.data.AnswerSalt)
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(m.data.AnswerHash)
	if err != nil || len(want) == 0 {
		return false
	}
	got := derive(strings.TrimSpace(answer), salt, m.data.Time, m.data.MemoryKiB, m.data.Threads, m.data.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (m *Manager) SetAutoLockMinutes(minutes int) error {
	if m == nil {
		return errno.New(errno.CodeUnknown, "applock.notReady", "")
	}
	if err := validateMinutes(minutes); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data.AutoLockMinutes = minutes
	return m.persistLocked()
}

func (m *Manager) Verify(password string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.verifyLocked(password)
}

func (m *Manager) verifyLocked(password string) bool {
	if m.data.Hash == "" || password == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(m.data.Salt)
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(m.data.Hash)
	if err != nil || len(want) == 0 {
		return false
	}
	got := derive(password, salt, m.data.Time, m.data.MemoryKiB, m.data.Threads, m.data.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (m *Manager) saveLocked(password string, minutes int, enabled bool, question, answer string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return errno.Wrap(errno.CodeUnknown, "applock.saltFailed", err)
	}
	hash := derive(password, salt, defaultTime, defaultMemory, defaultThreads, defaultKeyLen)
	previous := m.data
	m.data = fileState{
		Version:         1,
		Enabled:         enabled,
		AutoLockMinutes: minutes,
		Salt:            base64.StdEncoding.EncodeToString(salt),
		Hash:            base64.StdEncoding.EncodeToString(hash),
		Time:            defaultTime,
		MemoryKiB:       defaultMemory,
		Threads:         defaultThreads,
		KeyLen:          defaultKeyLen,
	}
	if question == "" || answer == "" {

		m.data.Question = previous.Question
		m.data.AnswerSalt = previous.AnswerSalt
		m.data.AnswerHash = previous.AnswerHash
	} else {
		answerSalt := make([]byte, saltLen)
		if _, err := rand.Read(answerSalt); err != nil {
			return errno.Wrap(errno.CodeUnknown, "applock.saltFailed", err)
		}
		answerHash := derive(answer, answerSalt, defaultTime, defaultMemory, defaultThreads, defaultKeyLen)
		m.data.Question = question
		m.data.AnswerSalt = base64.StdEncoding.EncodeToString(answerSalt)
		m.data.AnswerHash = base64.StdEncoding.EncodeToString(answerHash)
	}
	return m.persistLocked()
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "applock.writeFailed", err)
	}
	payload, err := json.Marshal(m.data)
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "applock.writeFailed", err)
	}
	tmp := fmt.Sprintf("%s.%d.tmp", m.path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "applock.writeFailed", err)
	}
	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return errno.Wrap(errno.CodeStoreFailed, "applock.writeFailed", err)
	}
	return nil
}

func derive(password string, salt []byte, timeCost, memoryKiB uint32, threads uint8, keyLen uint32) []byte {
	if timeCost == 0 || memoryKiB == 0 || threads == 0 || keyLen == 0 {

		timeCost, memoryKiB, threads, keyLen = defaultTime, defaultMemory, defaultThreads, defaultKeyLen
	}
	return argon2.IDKey([]byte(password), salt, timeCost, memoryKiB, threads, keyLen)
}

func validatePassword(password string) error {
	runes := []rune(password)
	if len(runes) < minPasswordLen {
		return errno.New(errno.CodeInvalidArg, "applock.passwordTooShort", "")
	}
	if len(runes) > maxPasswordLen {
		return errno.New(errno.CodeInvalidArg, "applock.passwordTooLong", "")
	}
	return nil
}

func validateMinutes(minutes int) error {
	if minutes < 0 || minutes > maxAutoLockMinutes {
		return errno.New(errno.CodeInvalidArg, "applock.invalidMinutes", "")
	}
	return nil
}
