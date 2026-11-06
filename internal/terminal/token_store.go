package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const tokenTTL = 2 * time.Minute

type tokenRecord struct {
	sessionID string
	expiresAt time.Time
}

type TokenStore struct {
	mu        sync.Mutex
	byToken   map[string]tokenRecord
	bySession map[string]string
	now       func() time.Time
	random    func([]byte) error
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		byToken:   make(map[string]tokenRecord),
		bySession: make(map[string]string),
		now:       time.Now,
		random: func(target []byte) error {
			_, err := rand.Read(target)
			return err
		},
	}
}

func (s *TokenStore) Issue(sessionID string) (string, error) {
	if s == nil || s.now == nil || s.random == nil || sessionID == "" {
		return "", errors.New("session stream token issuer is not ready")
	}
	var value [32]byte
	if err := s.random(value[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(value[:])

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.removeExpiredLocked(now)
	if _, exists := s.byToken[token]; exists {
		return "", errors.New("session stream token collision")
	}
	s.revokeLocked(sessionID)
	s.byToken[token] = tokenRecord{
		sessionID: sessionID,
		expiresAt: now.Add(tokenTTL),
	}
	s.bySession[sessionID] = token
	return token, nil
}

func (s *TokenStore) Consume(sessionID, token string) bool {
	if s == nil || s.now == nil || sessionID == "" || token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.removeExpiredLocked(now)
	record, ok := s.byToken[token]
	if !ok || record.sessionID != sessionID || !record.expiresAt.After(now) {
		return false
	}
	delete(s.byToken, token)
	delete(s.bySession, sessionID)
	return true
}

func (s *TokenStore) Revoke(sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokeLocked(sessionID)
}

func (s *TokenStore) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.byToken)
	clear(s.bySession)
}

func (s *TokenStore) revokeLocked(sessionID string) {
	if token := s.bySession[sessionID]; token != "" {
		delete(s.byToken, token)
		delete(s.bySession, sessionID)
	}
}

func (s *TokenStore) removeExpiredLocked(now time.Time) {
	for token, record := range s.byToken {
		if record.expiresAt.After(now) {
			continue
		}
		delete(s.byToken, token)
		if s.bySession[record.sessionID] == token {
			delete(s.bySession, record.sessionID)
		}
	}
}
