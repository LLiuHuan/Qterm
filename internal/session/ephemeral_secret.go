package session

import "sync"

const ephemeralSecretLimit = 32

type ephemeralSecrets struct {
	mu      sync.Mutex
	secrets map[string]ephemeralSecret

	seq uint64
}

type ephemeralSecret struct {
	value []byte
	seq   uint64
}

func newEphemeralSecrets() *ephemeralSecrets {
	return &ephemeralSecrets{secrets: make(map[string]ephemeralSecret)}
}

func (s *ephemeralSecrets) remember(sessionID string, secret []byte) {
	if s == nil || sessionID == "" || len(secret) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.secrets[sessionID]; !exists && len(s.secrets) >= ephemeralSecretLimit {
		s.evictOldestLocked()
	}
	if existing, ok := s.secrets[sessionID]; ok {
		wipeBytes(existing.value)
	}
	s.seq++
	s.secrets[sessionID] = ephemeralSecret{
		value: append([]byte(nil), secret...),
		seq:   s.seq,
	}
}

func (s *ephemeralSecrets) use(sessionID string, fn func(secret []byte) error) (bool, error) {
	if s == nil || sessionID == "" {
		return false, nil
	}
	s.mu.Lock()
	entry, ok := s.secrets[sessionID]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	secret := append([]byte(nil), entry.value...)
	s.mu.Unlock()

	defer wipeBytes(secret)
	if fn == nil {
		return true, nil
	}
	return true, fn(secret)
}

func (s *ephemeralSecrets) has(sessionID string) bool {
	if s == nil || sessionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secrets[sessionID]
	return ok
}

func (s *ephemeralSecrets) forget(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.secrets[sessionID]; ok {
		wipeBytes(entry.value)
		delete(s.secrets, sessionID)
	}
}

func (s *ephemeralSecrets) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, entry := range s.secrets {
		wipeBytes(entry.value)
		delete(s.secrets, id)
	}
}

func (s *ephemeralSecrets) evictOldestLocked() {
	var oldestID string
	var oldestSeq uint64
	for id, entry := range s.secrets {
		if oldestID == "" || entry.seq < oldestSeq {
			oldestID = id
			oldestSeq = entry.seq
		}
	}
	if oldestID == "" {
		return
	}
	wipeBytes(s.secrets[oldestID].value)
	delete(s.secrets, oldestID)
}

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
